//! SPEC §16 "Authorization Code Exchange", the refresh grant, and the token response —
//! including its RFC 8707 `resource` indicator.

use chrono::{DateTime, TimeDelta, Utc};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tokio::time::Instant;
use url::{Url, form_urlencoded};

use super::discovery::{SelectionError, SelectionFailure, ServerMetadata};
use super::pkce::Pkce;
use super::transport::{
    BodyFailure, TransportFailure, form_post, is_refused_redirect, network_failure,
    oauth_error_fields, read_within, send_within,
};
use super::{OAuthClient, is_blank};
use crate::error::{Error, ErrorCode, parse_retry_after};
use crate::http::header::RETRY_AFTER;
use crate::http::{HeaderMap, StatusCode};
use crate::security::require_secure_endpoint;
use crate::types::SensitiveString;

/// The most seconds a token's `expires_in` may claim: `i32::MAX`, the largest lifetime whose
/// `expires_at` arithmetic is safe in every SDK's runtime. A larger value is a malformed
/// response.
pub const MAX_TOKEN_LIFETIME_SECONDS: u64 = 2_147_483_647;

/// A token response, as the exchange, the refresh and the device grant all answer.
///
/// `expires_in` is a lifetime that starts decaying the moment the server answers, so
/// `expires_at` is worked out from it on arrival and is the field to keep. The two tokens
/// are [`SensitiveString`]s: serde-transparent, so a stored copy holds what the server
/// sent, but `[REDACTED]` under `{:?}`.
///
/// `resource` is the RFC 8707 indicator naming the account the token is bound to (BC5:
/// `urn:bc:account:<id>`). Echo it when refreshing — [`RefreshRequest::resource`] — since a
/// BC5 multi-account refresh token is refused without it.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Token {
    /// The bearer token.
    pub access_token: SensitiveString,
    /// The token to refresh with, when the server issued one.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub refresh_token: Option<SensitiveString>,
    /// `Bearer`, unless the server said otherwise.
    #[serde(default = "bearer")]
    pub token_type: String,
    /// The lifetime the server gave, in seconds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_in: Option<u64>,
    /// The scope the server granted, when it said.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scope: Option<String>,
    /// The RFC 8707 resource indicator the token is bound to, when the server said.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resource: Option<String>,
    /// When the token expires, worked out from `expires_in` on arrival.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
}

fn bearer() -> String {
    "Bearer".to_string()
}

impl Token {
    /// Whether the token has expired as of `now`. A token with no expiry never has.
    pub fn is_expired_at(&self, now: DateTime<Utc>) -> bool {
        self.expires_at.is_some_and(|expires_at| expires_at <= now)
    }
}

/// Trades an authorization code for tokens (`grant_type=authorization_code`).
#[derive(Debug, Clone, Default)]
pub struct ExchangeRequest {
    /// The token endpoint.
    pub token_endpoint: String,
    /// The code the authorization server sent back.
    pub code: SensitiveString,
    /// The redirect URI the authorization request named.
    pub redirect_uri: String,
    /// The client's identifier.
    pub client_id: String,
    /// The client's secret; sent only when given (a public client has none).
    pub client_secret: Option<SensitiveString>,
    /// The verifier [`generate_pkce`](super::generate_pkce) drew; sent only when given.
    pub code_verifier: Option<SensitiveString>,
}

/// Trades a refresh token for a new access token (`grant_type=refresh_token`).
#[derive(Debug, Clone, Default)]
pub struct RefreshRequest {
    /// The token endpoint.
    pub token_endpoint: String,
    /// The refresh token.
    pub refresh_token: SensitiveString,
    /// The client's identifier; sent only when non-empty.
    pub client_id: String,
    /// The client's secret; sent only when given.
    pub client_secret: Option<SensitiveString>,
    /// The RFC 8707 resource indicator to bind the refreshed token to; sent only when
    /// given. Echo the stored token's [`Token::resource`].
    pub resource: Option<String>,
}

/// The URL to send someone to so they can approve the client: `response_type=code`, PKCE
/// S256, `state`, and `scope` when given.
///
/// A device-only server has no `authorization_endpoint`, which is a
/// [`SelectionFailure::CapabilityUnavailable`] here.
pub fn authorization_url(
    metadata: &ServerMetadata,
    client_id: &str,
    redirect_uri: &str,
    scope: Option<&str>,
    state: &str,
    pkce: &Pkce,
) -> Result<Url, Error> {
    let endpoint = metadata
        .authorization_endpoint
        .as_deref()
        .filter(|endpoint| !endpoint.is_empty())
        .ok_or_else(|| {
            SelectionError::into_error(
                SelectionFailure::CapabilityUnavailable,
                "the authorization server has no authorization_endpoint".to_string(),
                None,
            )
        })?;
    let mut url = Url::parse(endpoint)
        .map_err(|_| Error::usage("authorization_endpoint is not a valid URL"))?;
    require_secure_endpoint(&url)?;
    {
        let mut query = url.query_pairs_mut();
        query.append_pair("response_type", "code");
        query.append_pair("client_id", client_id);
        query.append_pair("redirect_uri", redirect_uri);
        if let Some(scope) = scope {
            query.append_pair("scope", scope);
        }
        query.append_pair("state", state);
        query.append_pair("code_challenge", &pkce.challenge);
        query.append_pair("code_challenge_method", "S256");
    }
    Ok(url)
}

impl OAuthClient {
    /// SPEC §16's `exchangeCode`: POSTs the code to the token endpoint as a form and reads
    /// the token back.
    pub async fn exchange_code(&self, request: &ExchangeRequest) -> Result<Token, Error> {
        require(
            !request.token_endpoint.is_empty(),
            "token endpoint is required",
        )?;
        require(!is_blank(&request.code), "authorization code is required")?;
        require(!request.redirect_uri.is_empty(), "redirect URI is required")?;
        require(!request.client_id.is_empty(), "client ID is required")?;

        let form = {
            let mut form = form_urlencoded::Serializer::new(String::new());
            form.append_pair("grant_type", "authorization_code");
            form.append_pair("code", request.code.expose());
            form.append_pair("redirect_uri", &request.redirect_uri);
            form.append_pair("client_id", &request.client_id);
            if let Some(secret) = &request.client_secret {
                form.append_pair("client_secret", secret.expose());
            }
            if let Some(verifier) = &request.code_verifier {
                form.append_pair("code_verifier", verifier.expose());
            }
            form.finish()
        };
        self.post_token_form(&request.token_endpoint, form).await
    }

    /// SPEC §16's `refreshToken`: POSTs the refresh token to the token endpoint as a form
    /// and reads the rotated token back. `resource` goes with it only when set.
    pub async fn refresh_token(&self, request: &RefreshRequest) -> Result<Token, Error> {
        require(
            !request.token_endpoint.is_empty(),
            "token endpoint is required",
        )?;
        require(
            !is_blank(&request.refresh_token),
            "refresh token is required",
        )?;

        let form = {
            let mut form = form_urlencoded::Serializer::new(String::new());
            form.append_pair("grant_type", "refresh_token");
            form.append_pair("refresh_token", request.refresh_token.expose());
            if !request.client_id.is_empty() {
                form.append_pair("client_id", &request.client_id);
            }
            if let Some(secret) = &request.client_secret {
                form.append_pair("client_secret", secret.expose());
            }
            if let Some(resource) = &request.resource {
                form.append_pair("resource", resource);
            }
            form.finish()
        };
        self.post_token_form(&request.token_endpoint, form).await
    }

    /// One token-endpoint POST under the transport policy: HTTPS or localhost, the request
    /// timeout, a redirect refused off the status line before the body is touched, the body
    /// under its cap, and only a 200 yielding a token.
    async fn post_token_form(&self, token_endpoint: &str, form: String) -> Result<Token, Error> {
        let url = Url::parse(token_endpoint)
            .map_err(|_| Error::usage("token endpoint is not a valid URL"))?;
        require_secure_endpoint(&url)?;
        let request = form_post(&url, form)?;
        let deadline = Instant::now() + self.request_timeout;

        let response = send_within(self.http(), deadline, request)
            .await
            .map_err(|failure| network_failure(&url, &failure))?;
        let status = response.status();
        if is_refused_redirect(status) {
            return Err(Error::new(
                ErrorCode::ApiError,
                format!(
                    "redirect {} on the token endpoint is not followed",
                    status.as_u16()
                ),
            )
            .with_status(status.as_u16()));
        }
        let headers = response.headers().clone();
        let body = match read_within(deadline, response.into_body(), status).await {
            Ok(body) => body,
            Err(BodyFailure::TooLarge(error)) => return Err(error),
            Err(BodyFailure::TimedOut) => {
                return Err(network_failure(&url, &TransportFailure::TimedOut));
            }
            Err(BodyFailure::Failed(failure)) => {
                return Err(network_failure(&url, &TransportFailure::Failed(failure)));
            }
        };
        if status == StatusCode::OK {
            parse_token_response(&body, status, Utc::now())
        } else {
            Err(token_endpoint_error(status, &headers, &body))
        }
    }
}

fn require(condition: bool, message: &str) -> Result<(), Error> {
    if condition {
        Ok(())
    } else {
        Err(Error::usage(message))
    }
}

/// A non-200 from the token endpoint, rendered as its status and the RFC 6749 `error` and
/// `error_description` alone (SPEC §9).
///
/// The code follows what the caller can do about it: a grant the server no longer honours
/// (`invalid_grant`, `invalid_client`, `unauthorized_client`, `access_denied`, or a 401) is
/// `auth_required` — sign in again; a 429 is `rate_limit`; another 400 or 422 is
/// `validation`; a 5xx is a retryable `api_error`; anything else is `api_error`.
pub(super) fn token_endpoint_error(status: StatusCode, headers: &HeaderMap, body: &[u8]) -> Error {
    let code = status.as_u16();
    let fields = if status.is_redirection() {
        None
    } else {
        oauth_error_fields(body)
    };
    let (error_code, retryable) = match (code, fields.as_ref().map(|(name, _)| name.as_str())) {
        (_, Some("invalid_grant" | "invalid_client" | "unauthorized_client" | "access_denied")) => {
            (ErrorCode::AuthRequired, false)
        }
        (401, _) => (ErrorCode::AuthRequired, false),
        (429, _) => (ErrorCode::RateLimit, true),
        (400 | 422, _) => (ErrorCode::Validation, false),
        (500..=599, _) => (ErrorCode::ApiError, true),
        _ => (ErrorCode::ApiError, false),
    };
    let mut error = match fields {
        Some((name, description)) => {
            let error = Error::new(error_code, format!("token error: {name}"));
            match description {
                Some(description) => error.with_hint(description),
                None => error,
            }
        }
        None => Error::new(
            error_code,
            format!("token request failed with status {code}"),
        ),
    };
    error = error.with_status(code).retryable(retryable);
    if error.hint().is_none()
        && let Some(retry_after) = headers
            .get(RETRY_AFTER)
            .and_then(|value| value.to_str().ok())
            .and_then(|value| parse_retry_after(value, Utc::now()))
    {
        error = error.with_hint(format!("retry after {retry_after} seconds"));
    }
    error
}

/// SPEC §16's token-response rules, shared by the exchange, the refresh and the device
/// poll: a JSON object with a non-empty `access_token`; `token_type` absent or null is
/// `Bearer` and present must be non-empty; `refresh_token`, `scope` and `resource` absent
/// or null are absent and present must be strings, `resource` non-empty; `expires_in`
/// present must be a finite, positive, whole number of seconds no greater than
/// [`MAX_TOKEN_LIFETIME_SECONDS`]. Every refusal is `api_error` carrying `status`, and
/// renders none of the body.
pub(super) fn parse_token_response(
    body: &[u8],
    status: StatusCode,
    now: DateTime<Utc>,
) -> Result<Token, Error> {
    let malformed = |detail: &str| Error::malformed_response(detail).with_status(status.as_u16());
    let document = match serde_json::from_slice::<Value>(body) {
        Ok(Value::Object(document)) => document,
        Ok(_) => return Err(malformed("token response is not a JSON object")),
        Err(_) => return Err(malformed("token response is not JSON")),
    };
    let access_token = match document.get("access_token").and_then(Value::as_str) {
        Some(token) if !token.is_empty() => SensitiveString::new(token),
        _ => return Err(malformed("token response has no access_token")),
    };
    let token_type = match document.get("token_type") {
        None | Some(Value::Null) => bearer(),
        Some(Value::String(token_type)) if !token_type.is_empty() => token_type.clone(),
        Some(_) => {
            return Err(malformed(
                "token response token_type must be a non-empty string",
            ));
        }
    };
    let refresh_token = optional_string(&document, "refresh_token", false)
        .map_err(|detail| malformed(&detail))?
        .map(SensitiveString::new);
    let scope = optional_string(&document, "scope", false).map_err(|detail| malformed(&detail))?;
    let resource =
        optional_string(&document, "resource", true).map_err(|detail| malformed(&detail))?;
    let expires_in = match document.get("expires_in") {
        None | Some(Value::Null) => None,
        Some(Value::Number(number)) => Some(whole_seconds(number, MAX_TOKEN_LIFETIME_SECONDS).ok_or_else(
            || {
                malformed(&format!(
                    "token response expires_in must be a positive whole number of seconds no greater than {MAX_TOKEN_LIFETIME_SECONDS}"
                ))
            },
        )?),
        Some(_) => return Err(malformed("token response expires_in must be a number")),
    };
    let expires_at = expires_in.and_then(|seconds| {
        TimeDelta::try_seconds(i64::try_from(seconds).ok()?)
            .and_then(|lifetime| now.checked_add_signed(lifetime))
    });
    Ok(Token {
        access_token,
        refresh_token,
        token_type,
        expires_in,
        scope,
        resource,
        expires_at,
    })
}

/// A string member that may be absent or null, and when present must be a string —
/// non-empty when `non_empty`.
fn optional_string(
    document: &serde_json::Map<String, Value>,
    key: &str,
    non_empty: bool,
) -> Result<Option<String>, String> {
    match document.get(key) {
        None | Some(Value::Null) => Ok(None),
        Some(Value::String(text)) if non_empty && text.is_empty() => Err(format!(
            "token response {key} must be a non-empty string when present"
        )),
        Some(Value::String(text)) => Ok(Some(text.clone())),
        Some(_) => Err(format!("token response {key} must be a string")),
    }
}

/// A duration the server wrote as a JSON number: a positive whole number of seconds no
/// greater than `ceiling`, accepting an integer-valued float (`900.0`) and refusing a
/// fractional (`2.5`), non-positive, non-finite or oversized one.
pub(super) fn whole_seconds(number: &serde_json::Number, ceiling: u64) -> Option<u64> {
    if let Some(value) = number.as_u64() {
        return (value > 0 && value <= ceiling).then_some(value);
    }
    if number.as_i64().is_some() {
        return None;
    }
    let value = number.as_f64()?;
    #[allow(
        clippy::cast_precision_loss,
        clippy::cast_possible_truncation,
        clippy::cast_sign_loss
    )]
    {
        (value.is_finite() && value > 0.0 && value.fract() == 0.0 && value <= ceiling as f64)
            .then_some(value as u64)
    }
}

#[cfg(all(test, feature = "reqwest"))]
mod tests {
    use serde_json::json;
    use wiremock::matchers::{body_string_contains, header, method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    use super::super::generate_pkce;
    use super::super::transport::FORM_CONTENT_TYPE;
    use super::*;

    fn metadata(issuer: &str) -> ServerMetadata {
        ServerMetadata {
            issuer: issuer.to_string(),
            authorization_endpoint: Some(format!("{issuer}/authorize")),
            token_endpoint: format!("{issuer}/token"),
            ..ServerMetadata::default()
        }
    }

    fn exchange(server: &MockServer) -> ExchangeRequest {
        ExchangeRequest {
            token_endpoint: format!("{}/token", server.uri()),
            code: "code-1".into(),
            redirect_uri: "http://127.0.0.1:9000/callback".to_string(),
            client_id: "client-1".to_string(),
            client_secret: None,
            code_verifier: Some("verifier-1".into()),
        }
    }

    fn refresh(server: &MockServer) -> RefreshRequest {
        RefreshRequest {
            token_endpoint: format!("{}/token", server.uri()),
            refresh_token: "refresh-1".into(),
            client_id: "client-1".to_string(),
            client_secret: None,
            resource: None,
        }
    }

    fn form_pairs(body: &[u8]) -> Vec<(String, String)> {
        form_urlencoded::parse(body)
            .map(|(name, value)| (name.into_owned(), value.into_owned()))
            .collect()
    }

    #[test]
    fn the_authorization_url_carries_the_standard_parameters() {
        let pkce = generate_pkce();
        let url = authorization_url(
            &metadata("https://as.example"),
            "client-1",
            "http://127.0.0.1:9000/callback",
            Some("read write"),
            "state-1",
            &pkce,
        )
        .unwrap();
        let query: Vec<(String, String)> = url
            .query_pairs()
            .map(|(name, value)| (name.into_owned(), value.into_owned()))
            .collect();
        assert_eq!(url.path(), "/authorize");
        assert_eq!(
            query,
            vec![
                ("response_type".to_string(), "code".to_string()),
                ("client_id".to_string(), "client-1".to_string()),
                (
                    "redirect_uri".to_string(),
                    "http://127.0.0.1:9000/callback".to_string()
                ),
                ("scope".to_string(), "read write".to_string()),
                ("state".to_string(), "state-1".to_string()),
                ("code_challenge".to_string(), pkce.challenge.clone()),
                ("code_challenge_method".to_string(), "S256".to_string()),
            ]
        );
    }

    #[test]
    fn the_authorization_url_leaves_out_an_absent_scope() {
        let url = authorization_url(
            &metadata("https://as.example"),
            "client-1",
            "http://127.0.0.1:9000/callback",
            None,
            "state-1",
            &generate_pkce(),
        )
        .unwrap();
        assert!(!url.query().unwrap().contains("scope"));
    }

    #[test]
    fn a_device_only_server_has_no_authorization_url() {
        let device_only = ServerMetadata {
            authorization_endpoint: None,
            ..metadata("https://as.example")
        };
        let error = authorization_url(
            &device_only,
            "client-1",
            "http://127.0.0.1:9000/callback",
            None,
            "state-1",
            &generate_pkce(),
        )
        .unwrap_err();
        assert_eq!(error.code(), ErrorCode::Validation);
        assert_eq!(
            SelectionError::of(&error).unwrap().reason(),
            SelectionFailure::CapabilityUnavailable
        );
    }

    #[tokio::test]
    async fn exchange_trades_a_code_for_a_token() {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .and(path("/token"))
            .and(header("content-type", FORM_CONTENT_TYPE))
            .and(header("accept", "application/json"))
            .and(body_string_contains("grant_type=authorization_code"))
            .and(body_string_contains("code=code-1"))
            .and(body_string_contains("code_verifier=verifier-1"))
            .respond_with(ResponseTemplate::new(200).set_body_json(json!({
                "access_token": "access-1",
                "refresh_token": "refresh-1",
                "token_type": "Bearer",
                "expires_in": 3600,
                "scope": "read",
                "resource": "urn:bc:account:42"
            })))
            .mount(&server)
            .await;

        let token = OAuthClient::shipped()
            .unwrap()
            .exchange_code(&exchange(&server))
            .await
            .unwrap();

        assert_eq!(token.access_token.expose(), "access-1");
        assert_eq!(token.refresh_token.as_ref().unwrap().expose(), "refresh-1");
        assert_eq!(token.expires_in, Some(3600));
        assert_eq!(token.resource.as_deref(), Some("urn:bc:account:42"));
        assert!(token.expires_at.unwrap() > Utc::now());
        let body = server.received_requests().await.unwrap().remove(0).body;
        let pairs = form_pairs(&body);
        assert!(!pairs.iter().any(|(name, _)| name == "client_secret"));
    }

    #[tokio::test]
    async fn exchange_sends_the_secret_only_when_given() {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .respond_with(ResponseTemplate::new(200).set_body_json(json!({"access_token": "a"})))
            .mount(&server)
            .await;
        let request = ExchangeRequest {
            client_secret: Some("secret-1".into()),
            code_verifier: None,
            ..exchange(&server)
        };
        OAuthClient::shipped()
            .unwrap()
            .exchange_code(&request)
            .await
            .unwrap();
        let body = server.received_requests().await.unwrap().remove(0).body;
        let pairs = form_pairs(&body);
        assert!(pairs.contains(&("client_secret".to_string(), "secret-1".to_string())));
        assert!(!pairs.iter().any(|(name, _)| name == "code_verifier"));
    }

    #[tokio::test]
    async fn exchange_wants_its_required_fields() {
        let error = OAuthClient::shipped()
            .unwrap()
            .exchange_code(&ExchangeRequest::default())
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::Usage);
        assert_eq!(error.message(), "token endpoint is required");
    }

    #[tokio::test]
    async fn a_plain_http_token_endpoint_is_refused() {
        let request = RefreshRequest {
            token_endpoint: "http://as.example/token".to_string(),
            ..refresh(&MockServer::start().await)
        };
        let error = OAuthClient::shipped()
            .unwrap()
            .refresh_token(&request)
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::Usage);
    }

    #[tokio::test]
    async fn a_token_error_is_rendered_as_its_code_and_description_only() {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .respond_with(ResponseTemplate::new(400).set_body_json(json!({
                "error": "invalid_grant",
                "error_description": "The authorization code has expired",
                "echo": "code=code-1"
            })))
            .mount(&server)
            .await;
        let error = OAuthClient::shipped()
            .unwrap()
            .exchange_code(&exchange(&server))
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::AuthRequired);
        assert_eq!(error.message(), "token error: invalid_grant");
        assert_eq!(error.hint(), Some("The authorization code has expired"));
        assert_eq!(error.http_status(), Some(400));
        assert!(!error.to_string().contains("code-1"));
        assert!(error.body().is_none());
    }

    #[tokio::test]
    async fn an_unparsable_failure_body_is_never_rendered() {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .respond_with(ResponseTemplate::new(503).set_body_string("refresh_token=refresh-1"))
            .mount(&server)
            .await;
        let error = OAuthClient::shipped()
            .unwrap()
            .refresh_token(&refresh(&server))
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::ApiError);
        assert!(error.is_retryable());
        assert_eq!(error.message(), "token request failed with status 503");
        assert_eq!(error.hint(), None);
        assert!(!error.to_string().contains("refresh-1"));
    }

    #[tokio::test]
    async fn a_redirect_from_the_token_endpoint_is_not_followed() {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .and(path("/token"))
            .respond_with(
                ResponseTemplate::new(307)
                    .insert_header("location", format!("{}/elsewhere", server.uri()).as_str()),
            )
            .mount(&server)
            .await;
        let error = OAuthClient::shipped()
            .unwrap()
            .refresh_token(&refresh(&server))
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::ApiError);
        assert_eq!(error.http_status(), Some(307));
        assert!(
            error.message().contains("not followed"),
            "{}",
            error.message()
        );
        assert_eq!(server.received_requests().await.unwrap().len(), 1);
    }

    #[tokio::test]
    async fn a_304_is_the_generic_failure() {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .respond_with(ResponseTemplate::new(304))
            .mount(&server)
            .await;
        let error = OAuthClient::shipped()
            .unwrap()
            .refresh_token(&refresh(&server))
            .await
            .unwrap_err();
        assert_eq!(error.http_status(), Some(304));
        assert!(!error.message().contains("not followed"));
    }

    #[tokio::test]
    async fn a_stalled_token_endpoint_is_a_network_error_naming_the_origin_only() {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .respond_with(
                ResponseTemplate::new(200)
                    .set_delay(std::time::Duration::from_secs(5))
                    .set_body_json(json!({"access_token": "late"})),
            )
            .mount(&server)
            .await;
        let client = OAuthClient::shipped()
            .unwrap()
            .with_request_timeout(std::time::Duration::from_millis(200));
        let error = client.refresh_token(&refresh(&server)).await.unwrap_err();
        assert_eq!(error.code(), ErrorCode::Network);
        assert!(error.is_retryable());
        assert_eq!(
            error.message(),
            format!("Network error contacting {}", server.uri())
        );
    }

    #[tokio::test]
    async fn a_token_body_past_the_cap_is_refused() {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .respond_with(ResponseTemplate::new(200).set_body_bytes(vec![b'{'; 1_048_577]))
            .mount(&server)
            .await;
        let error = OAuthClient::shipped()
            .unwrap()
            .refresh_token(&refresh(&server))
            .await
            .unwrap_err();
        assert!(error.is_response_too_large());
        assert_eq!(error.code(), ErrorCode::ApiError);
    }

    #[test]
    fn timeouts_normalize_to_the_default_when_invalid() {
        let client = OAuthClient::shipped().unwrap();
        assert_eq!(
            client.request_timeout(),
            super::super::DEFAULT_REQUEST_TIMEOUT
        );
        assert_eq!(
            client
                .clone()
                .with_request_timeout(std::time::Duration::ZERO)
                .request_timeout(),
            super::super::DEFAULT_REQUEST_TIMEOUT
        );
        assert_eq!(
            client
                .clone()
                .with_request_timeout(std::time::Duration::from_secs(3601))
                .request_timeout(),
            super::super::DEFAULT_REQUEST_TIMEOUT
        );
        assert_eq!(
            client
                .with_request_timeout(std::time::Duration::from_secs(3600))
                .request_timeout(),
            std::time::Duration::from_secs(3600)
        );
    }

    #[test]
    fn a_token_response_is_validated_field_by_field() {
        let now = Utc::now();
        let parse = |body: &str| parse_token_response(body.as_bytes(), StatusCode::OK, now);

        let token = parse(r#"{"access_token":"a","expires_in":3600.0,"refresh_token":null,"scope":null,"resource":null}"#).unwrap();
        assert_eq!(token.token_type, "Bearer");
        assert_eq!(token.expires_in, Some(3600));
        assert_eq!(
            token.expires_at,
            now.checked_add_signed(TimeDelta::seconds(3600))
        );
        assert_eq!(token.refresh_token, None);
        assert_eq!(token.resource, None);

        assert_eq!(parse(r#"{"access_token":"a"}"#).unwrap().expires_at, None);

        for body in [
            "[]",
            "not json",
            r#"{"access_token":""}"#,
            r#"{"refresh_token":"r"}"#,
            r#"{"access_token":"a","token_type":""}"#,
            r#"{"access_token":"a","token_type":7}"#,
            r#"{"access_token":"a","refresh_token":7}"#,
            r#"{"access_token":"a","scope":[]}"#,
            r#"{"access_token":"a","resource":""}"#,
            r#"{"access_token":"a","resource":7}"#,
            r#"{"access_token":"a","expires_in":0}"#,
            r#"{"access_token":"a","expires_in":-1}"#,
            r#"{"access_token":"a","expires_in":2.5}"#,
            r#"{"access_token":"a","expires_in":"3600"}"#,
            r#"{"access_token":"a","expires_in":2147483648}"#,
            r#"{"access_token":"a","expires_in":1e400}"#,
        ] {
            let error = parse(body).unwrap_err();
            assert_eq!(error.code(), ErrorCode::ApiError, "{body}");
            assert_eq!(error.http_status(), Some(200), "{body}");
            assert!(
                !error.message().contains("\"a\""),
                "{body}: {}",
                error.message()
            );
        }
    }

    #[test]
    fn a_stored_token_round_trips_with_its_expiry() {
        let token = Token {
            access_token: "a".into(),
            refresh_token: Some("r".into()),
            token_type: "Bearer".to_string(),
            expires_in: Some(60),
            scope: None,
            resource: Some("urn:bc:account:1".to_string()),
            expires_at: Some(Utc::now()),
        };
        let stored = serde_json::to_string(&token).unwrap();
        assert!(!stored.contains("scope"));
        let restored: Token = serde_json::from_str(&stored).unwrap();
        assert_eq!(restored, token);
        assert_eq!(format!("{token:?}").matches("[REDACTED]").count(), 2);
    }

    /// The wire scenarios of `conformance/oauth-token/fixtures`, each a refresh against a
    /// wiremock token endpoint (see `conformance/oauth-token/README.md`).
    mod conformance {
        use std::path::PathBuf;

        use serde::Deserialize;

        use super::*;

        #[derive(Deserialize)]
        struct Request {
            resource: Option<String>,
        }

        #[derive(Deserialize)]
        struct Response {
            status: Option<u16>,
            body: Value,
        }

        #[derive(Deserialize)]
        #[serde(rename_all = "camelCase")]
        struct Expect {
            outcome: String,
            resource: Option<String>,
            resource_absent: Option<bool>,
            form_resource: Option<String>,
            form_resource_absent: Option<bool>,
        }

        #[derive(Deserialize)]
        struct Fixture {
            name: String,
            operation: String,
            request: Option<Request>,
            response: Response,
            expect: Expect,
        }

        fn fixture_paths() -> Vec<PathBuf> {
            let dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                .join("../../conformance/oauth-token/fixtures");
            let mut paths: Vec<PathBuf> = std::fs::read_dir(dir)
                .expect("the fixture directory exists")
                .map(|entry| entry.unwrap().path())
                .filter(|path| {
                    path.extension()
                        .is_some_and(|extension| extension == "json")
                })
                .collect();
            paths.sort();
            assert!(!paths.is_empty(), "no fixtures found");
            paths
        }

        async fn run(fixture_path: &PathBuf) {
            let fixture: Fixture =
                serde_json::from_str(&std::fs::read_to_string(fixture_path).unwrap()).unwrap();
            let name = fixture.name.as_str();
            assert_eq!(fixture.operation, "refreshToken", "{name}");

            let server = MockServer::start().await;
            Mock::given(method("POST"))
                .and(path("/token"))
                .respond_with(
                    ResponseTemplate::new(fixture.response.status.unwrap_or(200))
                        .set_body_json(&fixture.response.body),
                )
                .mount(&server)
                .await;

            let request = RefreshRequest {
                resource: fixture.request.and_then(|request| request.resource),
                ..refresh(&server)
            };
            let result = OAuthClient::shipped()
                .unwrap()
                .refresh_token(&request)
                .await;

            match fixture.expect.outcome.as_str() {
                "token" => {
                    let token = result.unwrap_or_else(|error| panic!("{name}: {error:?}"));
                    if let Some(want) = &fixture.expect.resource {
                        assert_eq!(token.resource.as_ref(), Some(want), "{name}");
                    }
                    if fixture.expect.resource_absent == Some(true) {
                        assert_eq!(token.resource, None, "{name}");
                    }
                }
                "reject" => {
                    let error = result
                        .err()
                        .unwrap_or_else(|| panic!("{name}: expected a rejection"));
                    assert_eq!(error.code(), ErrorCode::ApiError, "{name}");
                }
                other => panic!("{name}: unknown outcome {other}"),
            }

            let body = server.received_requests().await.unwrap().remove(0).body;
            let sent: Vec<(String, String)> = form_pairs(&body)
                .into_iter()
                .filter(|(key, _)| key == "resource")
                .collect();
            if let Some(want) = &fixture.expect.form_resource {
                assert_eq!(sent, vec![("resource".to_string(), want.clone())], "{name}");
            }
            if fixture.expect.form_resource_absent == Some(true) {
                assert!(sent.is_empty(), "{name}: sent {sent:?}");
            }
        }

        #[tokio::test]
        async fn every_token_fixture_holds() {
            for path in fixture_paths() {
                run(&path).await;
            }
        }
    }
}
