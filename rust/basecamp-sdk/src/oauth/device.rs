//! SPEC §16 "RFC 8628 Device Authorization Grant": the device-authorization request, the
//! token poll with its full CASE table, and the login that composes them.

use std::fmt;
use std::time::Duration;

use async_trait::async_trait;
use chrono::Utc;
use serde_json::Value;
use tokio::time::Instant;
use url::{Url, form_urlencoded};

use super::discovery::ServerMetadata;
use super::token::{Token, parse_token_response, whole_seconds};
use super::transport::{
    BodyFailure, TransportFailure, form_post, oauth_error_fields, read_within, send_within,
};
use super::{OAuthClient, is_blank};
use crate::error::{Error, ErrorCode};
use crate::http::header::RETRY_AFTER;
use crate::http::{HeaderMap, StatusCode};
use crate::security::{origin_of, require_secure_endpoint};
use crate::types::SensitiveString;

/// The RFC 8628 grant type, as an authorization server advertises it in
/// `grant_types_supported` and the poll sends it as `grant_type`.
pub const DEVICE_CODE_GRANT_TYPE: &str = "urn:ietf:params:oauth:grant-type:device_code";
/// The most seconds a device code's `expires_in`, its `interval`, or a `Retry-After` on the
/// poll may claim: the largest whole-second duration whose millisecond form fits a 32-bit
/// signed timer, shared by every SDK. A larger `expires_in` or `interval` is a malformed
/// response; a larger `Retry-After` clamps to it.
pub const MAX_DEVICE_SECONDS: u64 = 2_147_483;
const DEFAULT_INTERVAL_SECONDS: u64 = 5;
const SLOW_DOWN_INCREMENT_SECONDS: u64 = 5;
const MAX_BACKOFF_SECONDS: u64 = 60;

/// The monotonic time the device poll measures its deadline and its waits on. Injectable so
/// a test can run a flow in virtual time; [`MonotonicClock`] is the runtime's own.
#[async_trait]
pub trait Clock: Send + Sync {
    /// The moment now.
    fn now(&self) -> Instant;

    /// Waits `wait`. The default sleeps on the tokio timer, which a test drives with
    /// `tokio::time::pause` and `tokio::time::advance`.
    async fn sleep(&self, wait: Duration) {
        tokio::time::sleep(wait).await;
    }
}

/// [`tokio::time::Instant`], the runtime's monotonic clock.
#[derive(Debug, Clone, Copy, Default)]
pub struct MonotonicClock;

#[async_trait]
impl Clock for MonotonicClock {
    fn now(&self) -> Instant {
        Instant::now()
    }
}

/// An RFC 8628 §3.2 device authorization response: what to show the user, and what to poll
/// with. The device code redeems the token, so it prints as `[REDACTED]`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DeviceAuthorization {
    /// The code polled at the token endpoint.
    pub device_code: SensitiveString,
    /// The code the user types at `verification_uri`.
    pub user_code: String,
    /// Where the user enters the code.
    pub verification_uri: String,
    /// `verification_uri` with the code already in it, when the server gave one.
    pub verification_uri_complete: Option<String>,
    /// How many seconds the codes live.
    pub expires_in: u64,
    /// The least seconds between polls; 5 when the server did not say.
    pub interval: u64,
}

/// Why a device flow ended without a token.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum DeviceFlowReason {
    /// The user declined.
    AccessDenied,
    /// The device code expired before the user approved it.
    Expired,
    /// A transport failure ended the flow.
    Transport,
    /// The authorization server cannot do the device grant.
    Unavailable,
    /// The caller cancelled the flow. A Rust future is cancelled by dropping it, which
    /// returns nothing, so this reason is kept for the cross-SDK vocabulary and answered by
    /// no path here today.
    Cancelled,
}

impl DeviceFlowReason {
    /// The reason as every SDK spells it.
    pub fn as_str(&self) -> &'static str {
        match self {
            DeviceFlowReason::AccessDenied => "access_denied",
            DeviceFlowReason::Expired => "expired",
            DeviceFlowReason::Transport => "transport",
            DeviceFlowReason::Unavailable => "unavailable",
            DeviceFlowReason::Cancelled => "cancelled",
        }
    }

    /// The [`ErrorCode`] an error for this reason carries.
    pub fn error_code(&self) -> ErrorCode {
        match self {
            DeviceFlowReason::AccessDenied | DeviceFlowReason::Expired => ErrorCode::AuthRequired,
            DeviceFlowReason::Transport => ErrorCode::Network,
            DeviceFlowReason::Unavailable => ErrorCode::Validation,
            DeviceFlowReason::Cancelled => ErrorCode::Usage,
        }
    }

    fn message(self) -> &'static str {
        match self {
            DeviceFlowReason::AccessDenied => "the authorization request was denied",
            DeviceFlowReason::Expired => "the device code expired before authorization completed",
            DeviceFlowReason::Transport => "device flow transport failure",
            DeviceFlowReason::Unavailable => {
                "the authorization server does not support the device authorization grant"
            }
            DeviceFlowReason::Cancelled => "device flow cancelled",
        }
    }
}

impl fmt::Display for DeviceFlowReason {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// The typed reason behind a device flow's end, found on the [`Error`]'s source chain:
/// [`DeviceFlowError::of`] reads it back.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DeviceFlowError {
    reason: DeviceFlowReason,
}

impl DeviceFlowError {
    /// The reason.
    pub fn reason(&self) -> DeviceFlowReason {
        self.reason
    }

    /// The device flow reason `error` carries, when it carries one.
    pub fn of(error: &Error) -> Option<&DeviceFlowError> {
        std::error::Error::source(error).and_then(|source| source.downcast_ref())
    }

    /// An [`Error`] for `reason`, coded from it; a transport failure is retryable.
    pub(super) fn into_error(reason: DeviceFlowReason, detail: Option<String>) -> Error {
        let message = match detail {
            Some(detail) => format!("{}: {detail}", reason.message()),
            None => reason.message().to_string(),
        };
        Error::new(reason.error_code(), message)
            .retryable(reason == DeviceFlowReason::Transport)
            .with_source(DeviceFlowError { reason })
    }
}

impl fmt::Display for DeviceFlowError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.reason.as_str())
    }
}

impl std::error::Error for DeviceFlowError {}

/// One poll of the token endpoint, classified.
enum Poll {
    /// A 200 carrying a token.
    Token(Token),
    /// The request budget passed before an answer: back off and poll again.
    TimedOut,
    /// A failure that ends the flow, already typed.
    Failed(Error),
    /// A 4xx carrying an OAuth error code, or `http_<status>` when it carried none.
    Answer {
        code: String,
        status: StatusCode,
        retry_after: Option<u64>,
    },
}

impl OAuthClient {
    /// SPEC §16's `requestDeviceAuthorization`: POSTs `client_id` — and `scope` and
    /// `login_hint` only when given — to the device authorization endpoint and validates the
    /// codes it answers with.
    ///
    /// An omitted `scope` leaves the server to its default; `login_hint` is Basecamp's
    /// extension naming the user expected to approve, which steers the sign-in page and
    /// authenticates nothing. A transport failure is a [`DeviceFlowReason::Transport`]; a
    /// non-2xx is `api_error` with its status — 503 while BC5's grant is dark-launched.
    pub async fn request_device_authorization(
        &self,
        device_authorization_endpoint: &str,
        client_id: &str,
        scope: Option<&str>,
        login_hint: Option<&str>,
    ) -> Result<DeviceAuthorization, Error> {
        let url = Url::parse(device_authorization_endpoint)
            .map_err(|_| Error::usage("device authorization endpoint is not a valid URL"))?;
        require_secure_endpoint(&url)?;
        if client_id.is_empty() {
            return Err(Error::new(
                ErrorCode::Validation,
                "client ID is required for device authorization",
            ));
        }

        let form = {
            let mut form = form_urlencoded::Serializer::new(String::new());
            form.append_pair("client_id", client_id);
            if let Some(scope) = scope.filter(|scope| !scope.is_empty()) {
                form.append_pair("scope", scope);
            }
            if let Some(hint) = login_hint.filter(|hint| !hint.is_empty()) {
                form.append_pair("login_hint", hint);
            }
            form.finish()
        };
        let request = form_post(&url, form)?;
        let deadline = Instant::now() + self.request_timeout;

        let transport = |_: &TransportFailure| {
            DeviceFlowError::into_error(
                DeviceFlowReason::Transport,
                Some(format!("contacting {}", origin_of(url.as_str()))),
            )
        };
        let response = send_within(self.http(), deadline, request)
            .await
            .map_err(|failure| transport(&failure))?;
        let status = response.status();
        if !status.is_success() {
            return Err(Error::new(
                ErrorCode::ApiError,
                format!(
                    "device authorization failed with status {}",
                    status.as_u16()
                ),
            )
            .with_status(status.as_u16())
            .retryable(status.is_server_error()));
        }
        let body = match read_within(deadline, response.into_body(), status).await {
            Ok(body) => body,
            Err(BodyFailure::TooLarge(error)) => return Err(error),
            Err(BodyFailure::TimedOut) => return Err(transport(&TransportFailure::TimedOut)),
            Err(BodyFailure::Failed(error)) => {
                return Err(transport(&TransportFailure::Failed(error)));
            }
        };
        parse_device_authorization(&body, status)
    }

    /// SPEC §16's `pollDeviceToken`: polls the token endpoint with the device code until the
    /// user approves, declines, or the code expires.
    ///
    /// Every wait is the largest of the server's `interval` (grown by 5 s on each
    /// `slow_down`), the transient backoff a timed-out request doubles (to at most 60 s,
    /// reset by any completed round trip), and a one-shot `Retry-After` honoured on a 429
    /// `too_many_requests` — clamped to what is left of `expires_in`, measured on `clock`.
    /// Each request is bounded by the lesser of the request timeout and that remainder.
    /// Only a 200 yields a token; a 3xx, a 5xx or any other 2xx ends the flow as `api_error`
    /// before its body is read; a 4xx is read for its OAuth error code.
    ///
    /// `interval` and `expires_in` must be in `1..=`[`MAX_DEVICE_SECONDS`].
    pub async fn poll_device_token(
        &self,
        token_endpoint: &str,
        client_id: &str,
        device_code: &SensitiveString,
        interval: u64,
        expires_in: u64,
        clock: &dyn Clock,
    ) -> Result<Token, Error> {
        for (name, value) in [("expires_in", expires_in), ("interval", interval)] {
            if value == 0 || value > MAX_DEVICE_SECONDS {
                return Err(Error::usage(format!(
                    "{name} must be a positive number of seconds no greater than {MAX_DEVICE_SECONDS}"
                )));
            }
        }
        let deadline = clock.now() + Duration::from_secs(expires_in);
        self.poll_until(
            token_endpoint,
            client_id,
            device_code,
            interval,
            deadline,
            clock,
        )
        .await
    }

    /// SPEC §16's `performDeviceLogin`: the whole grant against an already-selected
    /// `config`. It requires a `device_authorization_endpoint` and the device grant among
    /// `grant_types_supported` — else [`DeviceFlowReason::Unavailable`], with no request
    /// made — then requests the codes, shows them through `display`, and polls for the
    /// token with whatever of the code's lifetime `display` left.
    pub async fn perform_device_login(
        &self,
        config: &ServerMetadata,
        client_id: &str,
        scope: Option<&str>,
        display: impl Fn(&DeviceAuthorization),
        clock: &dyn Clock,
        login_hint: Option<&str>,
    ) -> Result<Token, Error> {
        let supported = config
            .grant_types_supported
            .as_ref()
            .is_some_and(|grants| grants.iter().any(|grant| grant == DEVICE_CODE_GRANT_TYPE));
        let endpoint = match config.device_authorization_endpoint.as_deref() {
            Some(endpoint) if !endpoint.is_empty() && supported => endpoint,
            _ => {
                return Err(DeviceFlowError::into_error(
                    DeviceFlowReason::Unavailable,
                    None,
                ));
            }
        };

        let authorization = self
            .request_device_authorization(endpoint, client_id, scope, login_hint)
            .await?;
        let deadline = clock.now() + Duration::from_secs(authorization.expires_in);
        display(&authorization);
        if clock.now() >= deadline {
            return Err(DeviceFlowError::into_error(DeviceFlowReason::Expired, None));
        }
        self.poll_until(
            &config.token_endpoint,
            client_id,
            &authorization.device_code,
            authorization.interval,
            deadline,
            clock,
        )
        .await
    }

    async fn poll_until(
        &self,
        token_endpoint: &str,
        client_id: &str,
        device_code: &SensitiveString,
        interval: u64,
        deadline: Instant,
        clock: &dyn Clock,
    ) -> Result<Token, Error> {
        let url = Url::parse(token_endpoint)
            .map_err(|_| Error::usage("token endpoint is not a valid URL"))?;
        require_secure_endpoint(&url)?;
        if is_blank(device_code) {
            return Err(Error::usage("device code is required"));
        }
        let form = {
            let mut form = form_urlencoded::Serializer::new(String::new());
            form.append_pair("grant_type", DEVICE_CODE_GRANT_TYPE);
            form.append_pair("device_code", device_code.expose());
            form.append_pair("client_id", client_id);
            form.finish()
        };

        let mut interval = interval.max(1);
        let mut backoff = interval;
        let mut next_wait_override = 0;
        loop {
            let now = clock.now();
            if now >= deadline {
                return Err(DeviceFlowError::into_error(DeviceFlowReason::Expired, None));
            }
            let remaining = deadline - now;
            let wait = Duration::from_secs(interval.max(backoff).max(next_wait_override));
            next_wait_override = 0;
            clock.sleep(wait.min(remaining)).await;

            let now = clock.now();
            if now >= deadline {
                return Err(DeviceFlowError::into_error(DeviceFlowReason::Expired, None));
            }
            let budget = self.request_timeout.min(deadline - now);
            match self.post_device_token(&url, form.clone(), budget).await {
                Poll::Token(token) => return Ok(token),
                Poll::Failed(error) => return Err(error),
                Poll::TimedOut => backoff = (backoff * 2).min(MAX_BACKOFF_SECONDS),
                Poll::Answer {
                    code,
                    status,
                    retry_after,
                } => {
                    backoff = interval;
                    match code.as_str() {
                        "authorization_pending" => {}
                        "slow_down" => {
                            interval += SLOW_DOWN_INCREMENT_SECONDS;
                            backoff = interval;
                        }
                        "access_denied" => {
                            return Err(DeviceFlowError::into_error(
                                DeviceFlowReason::AccessDenied,
                                None,
                            ));
                        }
                        "expired_token" => {
                            return Err(DeviceFlowError::into_error(
                                DeviceFlowReason::Expired,
                                None,
                            ));
                        }
                        "too_many_requests" if status == StatusCode::TOO_MANY_REQUESTS => {
                            next_wait_override =
                                retry_after.map_or(interval, |seconds| seconds.max(interval));
                        }
                        other => {
                            return Err(Error::new(
                                ErrorCode::ApiError,
                                format!("device token request failed: {other}"),
                            )
                            .with_status(status.as_u16()));
                        }
                    }
                }
            }
        }
    }

    /// One poll, classified status-first so a response whose body stalls is judged by its
    /// status line rather than waited on.
    async fn post_device_token(&self, url: &Url, form: String, budget: Duration) -> Poll {
        let request = match form_post(url, form) {
            Ok(request) => request,
            Err(error) => return Poll::Failed(error),
        };
        let deadline = Instant::now() + budget;
        let transport = || {
            Poll::Failed(DeviceFlowError::into_error(
                DeviceFlowReason::Transport,
                Some(format!("contacting {}", origin_of(url.as_str()))),
            ))
        };
        let response = match send_within(self.http(), deadline, request).await {
            Ok(response) => response,
            Err(TransportFailure::TimedOut) => return Poll::TimedOut,
            Err(TransportFailure::Failed(_)) => return transport(),
        };
        let status = response.status();
        if status.is_redirection() {
            return Poll::Failed(
                Error::new(
                    ErrorCode::ApiError,
                    format!(
                        "redirect {} on the token endpoint is not followed",
                        status.as_u16()
                    ),
                )
                .with_status(status.as_u16()),
            );
        }
        if status != StatusCode::OK && !status.is_client_error() {
            return Poll::Failed(
                Error::new(
                    ErrorCode::ApiError,
                    format!(
                        "device token request failed with status {}",
                        status.as_u16()
                    ),
                )
                .with_status(status.as_u16()),
            );
        }
        let headers = response.headers().clone();
        let body = match read_within(deadline, response.into_body(), status).await {
            Ok(body) => body,
            Err(BodyFailure::TooLarge(error)) => return Poll::Failed(error),
            Err(BodyFailure::TimedOut) => return Poll::TimedOut,
            Err(BodyFailure::Failed(_)) => return transport(),
        };
        if status == StatusCode::OK {
            return match parse_token_response(&body, status, Utc::now()) {
                Ok(token) => Poll::Token(token),
                Err(error) => Poll::Failed(error),
            };
        }
        let mut code = oauth_error_fields(&body)
            .map_or_else(|| format!("http_{}", status.as_u16()), |(code, _)| code);
        if status == StatusCode::TOO_MANY_REQUESTS && code != "too_many_requests" {
            code = format!("http_{}", status.as_u16());
        }
        let retry_after = (status == StatusCode::TOO_MANY_REQUESTS)
            .then(|| retry_after_delta(&headers))
            .flatten();
        Poll::Answer {
            code,
            status,
            retry_after,
        }
    }
}

/// The one `Retry-After` of a 429 as delta-seconds: `1*DIGIT` around ASCII space and tab
/// only, positive, at most ten significant digits, clamped to [`MAX_DEVICE_SECONDS`]. An
/// HTTP-date, a sign, a fraction, a zero, a second header line or an overlong value is no
/// value, and the poll falls back to its interval.
fn retry_after_delta(headers: &HeaderMap) -> Option<u64> {
    let mut values = headers.get_all(RETRY_AFTER).iter();
    let value = values.next()?.to_str().ok()?;
    if values.next().is_some() {
        return None;
    }
    let trimmed = value.trim_matches([' ', '\t']);
    if trimmed.is_empty() || !trimmed.bytes().all(|byte| byte.is_ascii_digit()) {
        return None;
    }
    let significant = trimmed.trim_start_matches('0');
    if significant.len() > 10 {
        return None;
    }
    let seconds: u64 = significant.parse().ok()?;
    (seconds > 0).then(|| seconds.min(MAX_DEVICE_SECONDS))
}

fn parse_device_authorization(
    body: &[u8],
    status: StatusCode,
) -> Result<DeviceAuthorization, Error> {
    let malformed = |detail: &str| Error::malformed_response(detail).with_status(status.as_u16());
    let document = match serde_json::from_slice::<Value>(body) {
        Ok(Value::Object(document)) => document,
        Ok(_) => {
            return Err(malformed(
                "device authorization response is not a JSON object",
            ));
        }
        Err(_) => return Err(malformed("device authorization response is not JSON")),
    };
    let required = |key: &str| {
        document
            .get(key)
            .and_then(Value::as_str)
            .filter(|text| !text.is_empty())
            .map(str::to_string)
            .ok_or_else(|| malformed(&format!("device authorization response has no {key}")))
    };
    let device_code = SensitiveString::new(required("device_code")?);
    let user_code = required("user_code")?;
    let verification_uri = required("verification_uri")?;
    let verification_uri_complete = match document.get("verification_uri_complete") {
        None | Some(Value::Null) => None,
        Some(Value::String(uri)) => Some(uri.clone()),
        Some(_) => {
            return Err(malformed(
                "device authorization response verification_uri_complete must be a string",
            ));
        }
    };
    let duration = |key: &str| {
        let detail = format!(
            "device authorization response {key} must be a positive whole number of seconds no greater than {MAX_DEVICE_SECONDS}"
        );
        match document.get(key) {
            None | Some(Value::Null) => Ok(None),
            Some(Value::Number(number)) => whole_seconds(number, MAX_DEVICE_SECONDS)
                .map(Some)
                .ok_or_else(|| malformed(&detail)),
            Some(_) => Err(malformed(&detail)),
        }
    };
    let expires_in = duration("expires_in")?
        .ok_or_else(|| malformed("device authorization response has no expires_in"))?;
    let interval = duration("interval")?.unwrap_or(DEFAULT_INTERVAL_SECONDS);
    Ok(DeviceAuthorization {
        device_code,
        user_code,
        verification_uri,
        verification_uri_complete,
        expires_in,
        interval,
    })
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use serde_json::json;

    use super::super::testing::{Script, Scripted, form_pairs};
    use super::*;

    const TOKEN_ENDPOINT: &str = "https://as.example/oauth/token";
    const DEVICE_ENDPOINT: &str = "https://as.example/oauth/device";

    fn client(script: &Arc<Script>) -> OAuthClient {
        OAuthClient::new(Arc::clone(script))
    }

    fn pending() -> Scripted {
        Scripted::json(400, json!({"error": "authorization_pending"}))
    }

    fn token() -> Scripted {
        Scripted::json(200, json!({"access_token": "access-1", "expires_in": 3600}))
    }

    fn device_response() -> Value {
        json!({
            "device_code": "device-1",
            "user_code": "ABCD-EFGH",
            "verification_uri": "https://as.example/device",
            "expires_in": 900,
            "interval": 5
        })
    }

    fn reason_of(error: &Error) -> Option<DeviceFlowReason> {
        DeviceFlowError::of(error).map(DeviceFlowError::reason)
    }

    async fn poll(script: &Arc<Script>, interval: u64, expires_in: u64) -> Result<Token, Error> {
        client(script)
            .poll_device_token(
                TOKEN_ENDPOINT,
                "client-1",
                &SensitiveString::new("device-1"),
                interval,
                expires_in,
                &MonotonicClock,
            )
            .await
    }

    #[tokio::test(start_paused = true)]
    async fn device_authorization_sends_only_what_is_set() {
        let script = Script::new([Scripted::json(200, device_response())]);
        let authorization = client(&script)
            .request_device_authorization(DEVICE_ENDPOINT, "client-1", None, None)
            .await
            .unwrap();
        assert_eq!(authorization.device_code.expose(), "device-1");
        assert_eq!(authorization.interval, 5);
        assert_eq!(authorization.verification_uri_complete, None);
        assert_eq!(
            format!("{authorization:?}").matches("[REDACTED]").count(),
            1
        );
        let sent = script.requests();
        assert_eq!(
            form_pairs(&sent[0].body),
            vec![("client_id".to_string(), "client-1".to_string())]
        );

        let script = Script::new([Scripted::json(200, device_response())]);
        client(&script)
            .request_device_authorization(
                DEVICE_ENDPOINT,
                "client-1",
                Some("read"),
                Some("jane@example.com"),
            )
            .await
            .unwrap();
        assert_eq!(
            form_pairs(&script.requests()[0].body),
            vec![
                ("client_id".to_string(), "client-1".to_string()),
                ("scope".to_string(), "read".to_string()),
                ("login_hint".to_string(), "jane@example.com".to_string()),
            ]
        );
    }

    #[tokio::test(start_paused = true)]
    async fn device_authorization_is_validated() {
        let cases: Vec<(Value, Option<u64>)> = vec![
            (json!({"expires_in": 900.0, "interval": null}), Some(5)),
            (json!({"expires_in": 900, "interval": 7.0}), Some(7)),
            (json!({"expires_in": 2.5}), None),
            (json!({"expires_in": 1e100}), None),
            (json!({"expires_in": 0}), None),
            (json!({"expires_in": null}), None),
            (json!({"expires_in": 900, "interval": 2_147_484}), None),
            (json!({"expires_in": 900, "interval": "5"}), None),
            (json!({"expires_in": 900, "device_code": ""}), None),
            (json!({"expires_in": 900, "user_code": null}), None),
        ];
        for (overrides, want_interval) in cases {
            let mut body = device_response();
            for (key, value) in overrides.as_object().unwrap() {
                body[key] = value.clone();
            }
            let script = Script::new([Scripted::json(200, body.clone())]);
            let result = client(&script)
                .request_device_authorization(DEVICE_ENDPOINT, "client-1", None, None)
                .await;
            if let Some(interval) = want_interval {
                assert_eq!(result.unwrap().interval, interval, "{body}");
            } else {
                let error = result.unwrap_err();
                assert_eq!(error.code(), ErrorCode::ApiError, "{body}");
                assert_eq!(error.http_status(), Some(200), "{body}");
            }
        }
    }

    #[tokio::test(start_paused = true)]
    async fn device_authorization_refuses_by_status_before_the_body() {
        let script = Script::new([Scripted::json(
            503,
            json!({"error": "temporarily_unavailable"}),
        )]);
        let error = client(&script)
            .request_device_authorization(DEVICE_ENDPOINT, "client-1", None, None)
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::ApiError);
        assert_eq!(error.http_status(), Some(503));

        let error = client(&Script::new([]))
            .request_device_authorization("http://as.example/device", "client-1", None, None)
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::Usage);

        let error = client(&Script::new([]))
            .request_device_authorization(DEVICE_ENDPOINT, "", None, None)
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::Validation);

        let script = Script::new([Scripted::Fail]);
        let error = client(&script)
            .request_device_authorization(DEVICE_ENDPOINT, "client-1", None, None)
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::Network);
        assert!(error.is_retryable());
        assert_eq!(reason_of(&error), Some(DeviceFlowReason::Transport));
    }

    #[tokio::test(start_paused = true)]
    async fn the_poll_waits_the_interval_and_returns_the_token() {
        let script = Script::new([pending(), pending(), token()]);
        let token = poll(&script, 5, 900).await.unwrap();
        assert_eq!(token.access_token.expose(), "access-1");
        assert_eq!(script.seconds_at_each_request(), vec![5, 10, 15]);
        let pairs = form_pairs(&script.requests()[0].body);
        assert_eq!(
            pairs,
            vec![
                ("grant_type".to_string(), DEVICE_CODE_GRANT_TYPE.to_string()),
                ("device_code".to_string(), "device-1".to_string()),
                ("client_id".to_string(), "client-1".to_string()),
            ]
        );
    }

    #[tokio::test(start_paused = true)]
    async fn slow_down_grows_the_interval_for_good() {
        let script = Script::new([
            Scripted::json(400, json!({"error": "slow_down"})),
            pending(),
            token(),
        ]);
        poll(&script, 5, 900).await.unwrap();
        assert_eq!(script.seconds_at_each_request(), vec![5, 15, 25]);
    }

    #[tokio::test(start_paused = true)]
    async fn the_user_can_decline_or_let_the_code_expire() {
        let script = Script::new([Scripted::json(400, json!({"error": "access_denied"}))]);
        let error = poll(&script, 5, 900).await.unwrap_err();
        assert_eq!(error.code(), ErrorCode::AuthRequired);
        assert_eq!(reason_of(&error), Some(DeviceFlowReason::AccessDenied));

        let script = Script::new([Scripted::json(400, json!({"error": "expired_token"}))]);
        let error = poll(&script, 5, 900).await.unwrap_err();
        assert_eq!(error.code(), ErrorCode::AuthRequired);
        assert_eq!(reason_of(&error), Some(DeviceFlowReason::Expired));
    }

    #[tokio::test(start_paused = true)]
    async fn a_throttle_is_honoured_once_at_the_larger_of_interval_and_retry_after() {
        let throttled = |retry_after: Option<&str>| {
            let mut scripted = Scripted::json(429, json!({"error": "too_many_requests"}));
            if let Some(value) = retry_after {
                scripted = scripted.header("retry-after", value);
            }
            scripted
        };
        let script = Script::new([throttled(Some("30")), pending(), token()]);
        poll(&script, 5, 900).await.unwrap();
        assert_eq!(script.seconds_at_each_request(), vec![5, 35, 40]);

        for value in [
            None,
            Some("2"),
            Some("0"),
            Some("+30"),
            Some("30.5"),
            Some("Wed, 21 Oct 2015 07:28:00 GMT"),
            Some("00000000030000000000"),
        ] {
            let script = Script::new([throttled(value), token()]);
            poll(&script, 5, 900).await.unwrap();
            assert_eq!(script.seconds_at_each_request(), vec![5, 10], "{value:?}");
        }

        let script = Script::new([throttled(Some(" 0007\t")), token()]);
        poll(&script, 5, 900).await.unwrap();
        assert_eq!(script.seconds_at_each_request(), vec![5, 12]);

        let script = Script::new([throttled(Some("9999999999")), token()]);
        let error = poll(&script, 5, 20).await.unwrap_err();
        assert_eq!(reason_of(&error), Some(DeviceFlowReason::Expired));
        assert_eq!(script.seconds_at_each_request(), vec![5]);
    }

    #[tokio::test(start_paused = true)]
    async fn only_the_exact_throttle_pair_keeps_polling() {
        let script = Script::new([Scripted::json(
            429,
            json!({"error": "authorization_pending"}),
        )]);
        let error = poll(&script, 5, 900).await.unwrap_err();
        assert_eq!(error.code(), ErrorCode::ApiError);
        assert_eq!(error.http_status(), Some(429));
        assert!(error.message().contains("http_429"));

        let script = Script::new([Scripted::json(400, json!({"error": "too_many_requests"}))]);
        let error = poll(&script, 5, 900).await.unwrap_err();
        assert_eq!(error.code(), ErrorCode::ApiError);
        assert_eq!(error.http_status(), Some(400));
    }

    #[tokio::test(start_paused = true)]
    async fn a_timed_out_request_backs_off_and_a_completed_one_resets() {
        let script = Script::new([Scripted::Stall, Scripted::Stall, pending(), token()]);
        let client = client(&script).with_request_timeout(Duration::from_secs(3));
        client
            .poll_device_token(
                TOKEN_ENDPOINT,
                "client-1",
                &SensitiveString::new("device-1"),
                5,
                900,
                &MonotonicClock,
            )
            .await
            .unwrap();
        assert_eq!(script.seconds_at_each_request(), vec![5, 18, 41, 46]);
    }

    #[tokio::test(start_paused = true)]
    async fn a_request_near_expiry_is_bounded_by_the_remaining_lifetime() {
        let script = Script::new([Scripted::Stall]);
        let started = Instant::now();
        let error = poll(&script, 5, 7).await.unwrap_err();
        assert_eq!(reason_of(&error), Some(DeviceFlowReason::Expired));
        assert_eq!(started.elapsed().as_secs(), 7);
    }

    #[tokio::test(start_paused = true)]
    async fn the_deadline_is_checked_before_and_after_each_wait() {
        let script = Script::new([pending(), pending(), pending()]);
        let error = poll(&script, 5, 12).await.unwrap_err();
        assert_eq!(reason_of(&error), Some(DeviceFlowReason::Expired));
        assert_eq!(script.seconds_at_each_request(), vec![5, 10]);
    }

    #[tokio::test(start_paused = true)]
    async fn only_a_200_yields_a_token_and_a_redirect_is_terminal() {
        let cases = [
            Scripted::json(201, json!({"access_token": "a"})),
            Scripted::json(500, json!({"error": "authorization_pending"})),
            Scripted::json(302, json!({"error": "authorization_pending"}))
                .header("location", "https://evil.example/"),
            Scripted::json(200, json!({"error": "authorization_pending"})),
            Scripted::json(200, json!({"access_token": "a", "expires_in": 1.5})),
            Scripted::json(200, json!({"access_token": "a", "resource": ""})),
            Scripted::text(200, "not json"),
        ];
        for scripted in cases {
            let status = scripted.status();
            let script = Script::new([scripted]);
            let error = poll(&script, 5, 900).await.unwrap_err();
            assert_eq!(error.code(), ErrorCode::ApiError, "{status}");
            assert_eq!(error.http_status(), Some(status), "{status}");
            assert_eq!(script.requests().len(), 1, "{status}");
        }

        let script = Script::new([Scripted::Fail]);
        let error = poll(&script, 5, 900).await.unwrap_err();
        assert_eq!(error.code(), ErrorCode::Network);
        assert_eq!(reason_of(&error), Some(DeviceFlowReason::Transport));
    }

    #[tokio::test(start_paused = true)]
    async fn poll_arguments_are_bounded() {
        for (interval, expires_in) in [(0, 900), (5, 0), (5, MAX_DEVICE_SECONDS + 1)] {
            let error = poll(&Script::new([]), interval, expires_in)
                .await
                .unwrap_err();
            assert_eq!(error.code(), ErrorCode::Usage);
        }
    }

    fn device_config() -> ServerMetadata {
        ServerMetadata {
            issuer: "https://as.example".to_string(),
            token_endpoint: TOKEN_ENDPOINT.to_string(),
            device_authorization_endpoint: Some(DEVICE_ENDPOINT.to_string()),
            grant_types_supported: Some(vec![DEVICE_CODE_GRANT_TYPE.to_string()]),
            ..ServerMetadata::default()
        }
    }

    #[tokio::test(start_paused = true)]
    async fn login_requires_the_device_capability() {
        let without_endpoint = ServerMetadata {
            device_authorization_endpoint: None,
            ..device_config()
        };
        let without_grant = ServerMetadata {
            grant_types_supported: Some(vec!["refresh_token".to_string()]),
            ..device_config()
        };
        let empty_endpoint = ServerMetadata {
            device_authorization_endpoint: Some(String::new()),
            ..device_config()
        };
        for config in [without_endpoint, without_grant, empty_endpoint] {
            let script = Script::new([]);
            let error = client(&script)
                .perform_device_login(&config, "client-1", None, |_| {}, &MonotonicClock, None)
                .await
                .unwrap_err();
            assert_eq!(error.code(), ErrorCode::Validation);
            assert_eq!(reason_of(&error), Some(DeviceFlowReason::Unavailable));
            assert!(script.requests().is_empty());
        }
    }

    #[tokio::test(start_paused = true)]
    async fn login_shows_the_code_then_polls_with_what_the_display_left() {
        let script = Script::new([Scripted::json(200, device_response()), pending(), token()]);
        let shown = std::sync::Mutex::new(None);
        let token = client(&script)
            .perform_device_login(
                &device_config(),
                "client-1",
                Some("read"),
                |authorization| {
                    *shown.lock().unwrap() = Some(authorization.user_code.clone());
                },
                &MonotonicClock,
                None,
            )
            .await
            .unwrap();
        assert_eq!(token.access_token.expose(), "access-1");
        assert_eq!(shown.lock().unwrap().as_deref(), Some("ABCD-EFGH"));
        assert_eq!(script.seconds_at_each_request(), vec![0, 5, 10]);
    }

    #[tokio::test(start_paused = true)]
    async fn a_display_that_consumes_the_lifetime_expires_the_code() {
        let mut short = device_response();
        short["expires_in"] = json!(30);
        let script = Script::new([Scripted::json(200, short), token()]);
        let error = client(&script)
            .perform_device_login(
                &device_config(),
                "client-1",
                None,
                |_| std::thread::sleep(Duration::from_millis(1)),
                &SlowDisplayClock::default(),
                None,
            )
            .await
            .unwrap_err();
        assert_eq!(reason_of(&error), Some(DeviceFlowReason::Expired));
        assert_eq!(script.requests().len(), 1);
    }

    /// A clock that jumps 60 s on every read after the first, standing in for a display
    /// hook that took that long.
    #[derive(Default)]
    struct SlowDisplayClock {
        reads: std::sync::atomic::AtomicU64,
    }

    #[async_trait]
    impl Clock for SlowDisplayClock {
        fn now(&self) -> Instant {
            let reads = self.reads.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
            Instant::now() + Duration::from_secs(60 * reads)
        }
    }

    #[test]
    fn retry_after_deltas_are_strict() {
        let parse = |value: &str| {
            let mut headers = HeaderMap::new();
            headers.insert(RETRY_AFTER, value.parse().unwrap());
            retry_after_delta(&headers)
        };
        assert_eq!(parse("30"), Some(30));
        assert_eq!(parse(" \t30\t "), Some(30));
        assert_eq!(parse("0030"), Some(30));
        assert_eq!(parse("0"), None);
        assert_eq!(parse("-1"), None);
        assert_eq!(parse("1.5"), None);
        assert_eq!(parse("9999999999"), Some(MAX_DEVICE_SECONDS));
        assert_eq!(parse("99999999999"), None);
        assert_eq!(parse("Wed, 21 Oct 2015 07:28:00 GMT"), None);

        let mut headers = HeaderMap::new();
        headers.append(RETRY_AFTER, "30".parse().unwrap());
        headers.append(RETRY_AFTER, "40".parse().unwrap());
        assert_eq!(retry_after_delta(&headers), None);
    }
}
