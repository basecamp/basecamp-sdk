//! SPEC §16 "Resource-First Discovery": RFC 9728 resource metadata, RFC 8414 server
//! metadata with issuer binding, and the orchestrator that composes them under the
//! stage-sensitive fallback state machine.

use std::fmt;

use serde::{Deserialize, Serialize};
use serde_json::Value;
use tokio::time::Instant;
use url::Url;

use super::OAuthClient;
use super::transport::{BodyFailure, TransportFailure, json_get, read_within, send_within};
use crate::error::{Error, ErrorCode};
use crate::http::StatusCode;
use crate::security::{is_localhost, origin_of};

/// Basecamp's Launchpad authorization server, the fallback when a resource advertises no
/// issuer of its own.
pub const LAUNCHPAD_ISSUER: &str = "https://launchpad.37signals.com";

const WELL_KNOWN_AS: &str = "/.well-known/oauth-authorization-server";
const WELL_KNOWN_RESOURCE: &str = "/.well-known/oauth-protected-resource";
const LIST_FIELDS: &[&str] = &[
    "grant_types_supported",
    "scopes_supported",
    "code_challenge_methods_supported",
];

/// What an authorization server publishes about itself (RFC 8414).
///
/// `token_endpoint` is the one endpoint every server has. `authorization_endpoint` is
/// absent on a device-only server, so a consumer of the authorization-code grant asserts
/// it before use — [`authorization_url`](super::authorization_url) does — and the device
/// grant asserts `device_authorization_endpoint` and its grant type the same way.
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
pub struct ServerMetadata {
    /// The issuer identifier: the origin the metadata was read from, code point for code
    /// point.
    pub issuer: String,
    /// Where a user is sent to approve the client. Absent on a device-only server.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub authorization_endpoint: Option<String>,
    /// Where codes and refresh tokens are traded for tokens.
    pub token_endpoint: String,
    /// The RFC 8628 device authorization endpoint, when the server has one.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub device_authorization_endpoint: Option<String>,
    /// The dynamic client registration endpoint, when the server has one.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub registration_endpoint: Option<String>,
    /// The grant types the server supports, when it says.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub grant_types_supported: Option<Vec<String>>,
    /// The scopes the server supports, when it says.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scopes_supported: Option<Vec<String>>,
    /// The PKCE challenge methods the server supports, when it says.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub code_challenge_methods_supported: Option<Vec<String>>,
}

/// What a protected resource publishes about who may issue tokens for it (RFC 9728).
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
pub struct ProtectedResourceMetadata {
    /// The resource identifier: the origin asked, code point for code point.
    pub resource: String,
    /// The issuers advertised for the resource. `None` when the key was absent — BC5's
    /// posture while its authorization server is dark — as distinct from `Some(vec![])`;
    /// both select Launchpad, but a caller reading the metadata can tell them apart.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub authorization_servers: Option<Vec<String>>,
}

/// Why [`OAuthClient::discover_from_resource`] answered with Launchpad instead of a
/// selected issuer. These two are the only soft outcomes; every other failure raises.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum FallbackReason {
    /// The resource metadata could not be fetched, parsed or bound, before any issuer was
    /// committed to.
    ResourceDiscoveryFailed,
    /// Valid resource metadata advertised no issuer but Launchpad: the key was absent, the
    /// list empty, or Launchpad its only member.
    NoAsAdvertised,
}

impl FallbackReason {
    /// The reason as every SDK spells it.
    pub fn as_str(&self) -> &'static str {
        match self {
            FallbackReason::ResourceDiscoveryFailed => "resource_discovery_failed",
            FallbackReason::NoAsAdvertised => "no_as_advertised",
        }
    }
}

impl fmt::Display for FallbackReason {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// What resource-first discovery settled on.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum DiscoveryOutcome {
    /// An advertised issuer was selected and its metadata bound; `issuer` is the advertised
    /// spelling.
    Selected(ServerMetadata),
    /// No issuer could be selected before one was committed to; the caller's next step is
    /// Launchpad, via [`OAuthClient::discover_launchpad`].
    Fallback(FallbackReason),
}

impl DiscoveryOutcome {
    /// The selected metadata, when an issuer was selected.
    pub fn selected(&self) -> Option<&ServerMetadata> {
        match self {
            DiscoveryOutcome::Selected(metadata) => Some(metadata),
            DiscoveryOutcome::Fallback(_) => None,
        }
    }

    /// The fallback reason, when discovery fell back.
    pub fn fallback(&self) -> Option<FallbackReason> {
        match self {
            DiscoveryOutcome::Selected(_) => None,
            DiscoveryOutcome::Fallback(reason) => Some(*reason),
        }
    }
}

/// The hard failures of resource-first selection. None of them may be turned into a
/// Launchpad request by a consumer: once a resource has advertised an issuer, that issuer
/// is the only one its tokens may come from.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum SelectionFailure {
    /// Two or more non-Launchpad issuers advertised and no expected issuer to choose by.
    AmbiguousIssuers,
    /// The expected issuer is not among those advertised.
    ExpectedIssuerUnavailable,
    /// The selected issuer is not a valid origin root.
    InvalidIssuerOrigin,
    /// The selected issuer's metadata could not be fetched.
    AsFetchFailed,
    /// The selected issuer's metadata names another issuer.
    IssuerMismatch,
    /// The selected issuer lacks an endpoint or grant type the consumer needs.
    CapabilityUnavailable,
}

impl SelectionFailure {
    /// The failure as every SDK spells it.
    pub fn as_str(&self) -> &'static str {
        match self {
            SelectionFailure::AmbiguousIssuers => "ambiguous_issuers",
            SelectionFailure::ExpectedIssuerUnavailable => "expected_issuer_unavailable",
            SelectionFailure::InvalidIssuerOrigin => "invalid_issuer_origin",
            SelectionFailure::AsFetchFailed => "as_fetch_failed",
            SelectionFailure::IssuerMismatch => "issuer_mismatch",
            SelectionFailure::CapabilityUnavailable => "capability_unavailable",
        }
    }

    /// The [`ErrorCode`] an error for this failure carries: `api_error` for every hard
    /// discovery failure, `validation` for the consumer-asserted capability.
    pub fn error_code(&self) -> ErrorCode {
        match self {
            SelectionFailure::CapabilityUnavailable => ErrorCode::Validation,
            _ => ErrorCode::ApiError,
        }
    }
}

impl fmt::Display for SelectionFailure {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// The typed reason behind a hard selection failure, found on the [`Error`]'s source
/// chain: [`SelectionError::of`] reads it back.
#[derive(Debug)]
pub struct SelectionError {
    reason: SelectionFailure,
    cause: Option<Error>,
}

impl SelectionError {
    /// The failure.
    pub fn reason(&self) -> SelectionFailure {
        self.reason
    }

    /// The error underneath, when the failure has one — the fetch that failed, the
    /// origin-root check that refused.
    pub fn cause(&self) -> Option<&Error> {
        self.cause.as_ref()
    }

    /// The selection failure `error` carries, when it carries one.
    pub fn of(error: &Error) -> Option<&SelectionError> {
        std::error::Error::source(error).and_then(|source| source.downcast_ref())
    }

    /// An [`Error`] for `reason`, coded from it, inheriting the status, retryability and
    /// request id of `cause` so an `api_error` over a 503 still says 503.
    pub(super) fn into_error(
        reason: SelectionFailure,
        message: String,
        cause: Option<Error>,
    ) -> Error {
        let mut error = Error::new(reason.error_code(), message);
        if let Some(cause) = &cause {
            if let Some(status) = cause.http_status() {
                error = error.with_status(status);
            }
            if let Some(request_id) = cause.request_id() {
                error = error.with_request_id(request_id);
            }
            error = error.retryable(cause.is_retryable());
        }
        error.with_source(SelectionError { reason, cause })
    }
}

impl fmt::Display for SelectionError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.reason.as_str())
    }
}

impl std::error::Error for SelectionError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        self.cause
            .as_ref()
            .map(|cause| cause as &(dyn std::error::Error + 'static))
    }
}

/// SPEC §16's `requireOriginRoot`: the origin `raw` names, as `scheme://host[:port]`, or a
/// usage error.
///
/// `raw` must be `https`, or `http` on localhost; carry a host; carry no path beyond `/`,
/// no query, no fragment and no userinfo; and name a port, if it names one, as plain digits
/// in `1..=65535`. The raw text is judged before the URL parser sees it — a parser
/// normalizes a dot segment, an empty fragment or a dangling `:` away — and the parser
/// settles what the text left open: the host, and whether it is one the client would dial.
/// Bracketed IPv6 is accepted (`http://[::1]:3000`); a default port is dropped from the
/// answer, and the host is lowercased.
pub fn require_origin_root(raw: &str) -> Result<String, Error> {
    let shown = origin_of(raw);
    let refuse = |what: &str| Error::usage(format!("{what}: {shown}"));

    if raw
        .chars()
        .any(|character| character <= ' ' || character == '\\')
    {
        return Err(refuse("origin contains invalid characters"));
    }
    let Some((_, after_scheme)) = raw.split_once("://") else {
        return Err(refuse("origin must be an absolute URL with an authority"));
    };
    if raw.contains(['?', '#']) {
        return Err(refuse("origin must not carry a query or fragment"));
    }
    let (authority, path) = after_scheme
        .split_once('/')
        .map_or((after_scheme, ""), |(authority, rest)| (authority, rest));
    if !path.is_empty() {
        return Err(refuse("origin must not carry a path"));
    }
    if authority.is_empty() {
        return Err(refuse("origin has no host"));
    }
    if authority.contains('@') {
        return Err(refuse("origin must not carry userinfo"));
    }
    let port = if let Some(rest) = authority.strip_prefix('[') {
        rest.split_once(']')
            .and_then(|(_, after)| after.strip_prefix(':'))
    } else {
        authority.rsplit_once(':').map(|(_, port)| port)
    };
    if let Some(port) = port {
        let valid = !port.is_empty()
            && port.bytes().all(|byte| byte.is_ascii_digit())
            && port
                .parse::<u32>()
                .is_ok_and(|number| (1..=65535).contains(&number));
        if !valid {
            return Err(refuse("origin has an invalid port"));
        }
    }

    let url = Url::parse(raw).map_err(|_| refuse("origin is not a valid URL"))?;
    let Some(host) = url.host_str() else {
        return Err(refuse("origin has no host"));
    };
    let secure = url.scheme() == "https" || (url.scheme() == "http" && is_localhost(&url));
    if !secure {
        return Err(refuse("origin must use HTTPS, or HTTP on localhost"));
    }
    let port = url
        .port()
        .map(|port| format!(":{port}"))
        .unwrap_or_default();
    Ok(format!("{}://{host}{port}", url.scheme()))
}

/// Why the server-metadata fetch of a committed issuer failed, kept apart so the
/// orchestrator can name `issuer_mismatch` without reading message text.
enum MetadataFailure {
    IssuerMismatch(Error),
    Other(Error),
}

impl MetadataFailure {
    fn into_error(self) -> Error {
        match self {
            MetadataFailure::IssuerMismatch(error) | MetadataFailure::Other(error) => error,
        }
    }
}

impl OAuthClient {
    /// RFC 8414 discovery: reads `{issuer}/.well-known/oauth-authorization-server` and binds
    /// it — the document's `issuer` must equal `issuer` code point for code point.
    ///
    /// `issuer` must be an origin root ([`require_origin_root`]); it is fetched from the
    /// normalized origin but bound against the string given, so `https://as.example/`
    /// binds to a document saying exactly that. `token_endpoint` must be present; any
    /// `*_endpoint` present must be a non-empty string; any `*_supported` list present must
    /// be an array of strings. `authorization_endpoint` is not required here — each grant's
    /// consumer asserts the endpoints it needs.
    pub async fn discover(&self, issuer: &str) -> Result<ServerMetadata, Error> {
        let origin = require_origin_root(issuer)?;
        self.fetch_server_metadata(&origin, issuer)
            .await
            .map_err(MetadataFailure::into_error)
    }

    /// [`OAuthClient::discover`] at the Launchpad issuer.
    pub async fn discover_launchpad(&self) -> Result<ServerMetadata, Error> {
        let issuer = self.launchpad_issuer.clone();
        self.discover(&issuer).await
    }

    /// RFC 9728 discovery: reads `{resource}/.well-known/oauth-protected-resource` and
    /// binds it — the document's `resource` must equal `resource` code point for code point.
    ///
    /// `resource` must be an origin root. `authorization_servers` is optional and, when
    /// present, must be an array of strings; absent and empty are kept distinct.
    pub async fn discover_protected_resource(
        &self,
        resource: &str,
    ) -> Result<ProtectedResourceMetadata, Error> {
        let origin = require_origin_root(resource)?;
        self.fetch_protected_resource(&origin, resource).await
    }

    /// Resource-first discovery, SPEC §16's `discoverFromResource`: reads the resource's
    /// metadata, selects an issuer from what it advertises, and reads and binds that
    /// issuer's metadata.
    ///
    /// With `expected_issuer`, the advertised member equal to it is selected and any other
    /// outcome is [`SelectionFailure::ExpectedIssuerUnavailable`]. Without it, the one
    /// advertised issuer that is not Launchpad is selected; two or more are
    /// [`SelectionFailure::AmbiguousIssuers`], and none is a
    /// [`FallbackReason::NoAsAdvertised`] fallback.
    ///
    /// Fallback is possible only before an issuer is committed to: a resource fetch that
    /// fails, or a resource that advertises nothing but Launchpad, answers
    /// [`DiscoveryOutcome::Fallback`]. Once an issuer is selected, every later failure — a
    /// bad origin, a fetch that fails, an issuer that does not bind — is an error carrying a
    /// [`SelectionError`], and no consumer may answer it with a Launchpad request.
    pub async fn discover_from_resource(
        &self,
        resource: &str,
        expected_issuer: Option<&str>,
    ) -> Result<DiscoveryOutcome, Error> {
        let origin = require_origin_root(resource)?;

        let Ok(metadata) = self.fetch_protected_resource(&origin, resource).await else {
            return Ok(DiscoveryOutcome::Fallback(
                FallbackReason::ResourceDiscoveryFailed,
            ));
        };
        let advertised = metadata.authorization_servers.unwrap_or_default();

        let selected = if let Some(expected) = expected_issuer {
            match advertised.iter().find(|issuer| *issuer == expected) {
                Some(issuer) => issuer.clone(),
                None => {
                    return Err(SelectionError::into_error(
                        SelectionFailure::ExpectedIssuerUnavailable,
                        format!(
                            "expected issuer {} is not advertised by the resource",
                            origin_of(expected)
                        ),
                        None,
                    ));
                }
            }
        } else {
            let mut candidates: Vec<&String> = Vec::new();
            for issuer in &advertised {
                if !self.is_launchpad_issuer(issuer) && !candidates.contains(&issuer) {
                    candidates.push(issuer);
                }
            }
            match candidates.as_slice() {
                [] => return Ok(DiscoveryOutcome::Fallback(FallbackReason::NoAsAdvertised)),
                [only] => (*only).clone(),
                many => {
                    let named: Vec<String> = many.iter().map(|issuer| origin_of(issuer)).collect();
                    return Err(SelectionError::into_error(
                        SelectionFailure::AmbiguousIssuers,
                        format!(
                            "the resource advertises {} non-Launchpad issuers ({}); name the expected issuer",
                            many.len(),
                            named.join(", ")
                        ),
                        None,
                    ));
                }
            }
        };

        let issuer_origin = match require_origin_root(&selected) {
            Ok(issuer_origin) => issuer_origin,
            Err(cause) => {
                return Err(SelectionError::into_error(
                    SelectionFailure::InvalidIssuerOrigin,
                    format!(
                        "advertised issuer {} is not a valid origin root",
                        origin_of(&selected)
                    ),
                    Some(cause),
                ));
            }
        };
        match self.fetch_server_metadata(&issuer_origin, &selected).await {
            Ok(metadata) => Ok(DiscoveryOutcome::Selected(metadata)),
            Err(MetadataFailure::IssuerMismatch(cause)) => Err(SelectionError::into_error(
                SelectionFailure::IssuerMismatch,
                format!("metadata at {issuer_origin} names another issuer than the advertised one"),
                Some(cause),
            )),
            Err(MetadataFailure::Other(cause)) => Err(SelectionError::into_error(
                SelectionFailure::AsFetchFailed,
                format!(
                    "authorization server metadata could not be read from the committed issuer {issuer_origin}: {}",
                    cause.message()
                ),
                Some(cause),
            )),
        }
    }

    /// Whether an advertised issuer denotes Launchpad: the same origin root, however it is
    /// spelled. A Launchpad look-alike with a path is not an origin root and so is not
    /// Launchpad.
    fn is_launchpad_issuer(&self, issuer: &str) -> bool {
        match (
            require_origin_root(issuer),
            require_origin_root(&self.launchpad_issuer),
        ) {
            (Ok(candidate), Ok(launchpad)) => candidate == launchpad,
            _ => false,
        }
    }

    async fn fetch_server_metadata(
        &self,
        origin: &str,
        bind_issuer: &str,
    ) -> Result<ServerMetadata, MetadataFailure> {
        let (status, body) = self
            .fetch_document(&format!("{origin}{WELL_KNOWN_AS}"))
            .await
            .map_err(MetadataFailure::Other)?;
        parse_server_metadata(&body, bind_issuer, status)
    }

    async fn fetch_protected_resource(
        &self,
        origin: &str,
        bind_resource: &str,
    ) -> Result<ProtectedResourceMetadata, Error> {
        let (status, body) = self
            .fetch_document(&format!("{origin}{WELL_KNOWN_RESOURCE}"))
            .await?;
        parse_protected_resource(&body, bind_resource, status)
    }

    /// One SSRF-hardened GET: the origin was validated before this, the transport follows no
    /// redirect, the request is bounded by the client's timeout, and the body is read under
    /// the cap. A non-2xx is `api_error`; a body past the cap is `api_error`; a transport
    /// failure is the [`crate::http::HttpClient`]'s `network` error.
    async fn fetch_document(&self, url: &str) -> Result<(StatusCode, bytes::Bytes), Error> {
        let deadline = Instant::now() + self.request_timeout;
        let request = json_get(url)?;
        let response = match send_within(self.http(), deadline, request).await {
            Ok(response) => response,
            Err(TransportFailure::TimedOut) => {
                return Err(Error::network_at(&origin_of(url)).with_hint("the request timed out"));
            }
            Err(TransportFailure::Failed(error)) => return Err(error),
        };
        let status = response.status();
        if !status.is_success() {
            return Err(Error::new(
                ErrorCode::ApiError,
                format!(
                    "OAuth discovery at {} failed with status {}",
                    origin_of(url),
                    status.as_u16()
                ),
            )
            .with_status(status.as_u16())
            .retryable(status.is_server_error()));
        }
        match read_within(deadline, response.into_body(), status).await {
            Ok(body) => Ok((status, body)),
            Err(BodyFailure::TooLarge(error) | BodyFailure::Failed(error)) => Err(error),
            Err(BodyFailure::TimedOut) => {
                Err(Error::network_at(&origin_of(url)).with_hint("the response timed out"))
            }
        }
    }
}

fn invalid_metadata(status: StatusCode, detail: &str) -> Error {
    Error::malformed_response(detail).with_status(status.as_u16())
}

fn parse_server_metadata(
    body: &[u8],
    bind_issuer: &str,
    status: StatusCode,
) -> Result<ServerMetadata, MetadataFailure> {
    let document = parse_object(body, status, "authorization server metadata")
        .map_err(MetadataFailure::Other)?;

    let issuer = match document.get("issuer").and_then(Value::as_str) {
        Some(issuer) if !issuer.is_empty() => issuer,
        _ => {
            return Err(MetadataFailure::Other(invalid_metadata(
                status,
                "authorization server metadata has no issuer",
            )));
        }
    };
    if issuer != bind_issuer {
        return Err(MetadataFailure::IssuerMismatch(invalid_metadata(
            status,
            &format!(
                "authorization server metadata issuer {} does not equal the issuer it was read for, {}",
                origin_of(issuer),
                origin_of(bind_issuer)
            ),
        )));
    }
    for (key, value) in &document {
        if key.ends_with("_endpoint") && value.as_str().is_none_or(str::is_empty) {
            return Err(MetadataFailure::Other(invalid_metadata(
                status,
                &format!("authorization server metadata {key} must be a non-empty string"),
            )));
        }
    }
    if !document.contains_key("token_endpoint") {
        return Err(MetadataFailure::Other(invalid_metadata(
            status,
            "authorization server metadata has no token_endpoint",
        )));
    }
    for key in LIST_FIELDS {
        if document
            .get(*key)
            .is_some_and(|value| !is_string_array(value))
        {
            return Err(MetadataFailure::Other(invalid_metadata(
                status,
                &format!("authorization server metadata {key} must be an array of strings"),
            )));
        }
    }
    serde_json::from_value(Value::Object(document)).map_err(|error| {
        MetadataFailure::Other(invalid_metadata(
            status,
            &format!("authorization server metadata: {error}"),
        ))
    })
}

fn parse_protected_resource(
    body: &[u8],
    bind_resource: &str,
    status: StatusCode,
) -> Result<ProtectedResourceMetadata, Error> {
    let document = parse_object(body, status, "resource metadata")?;
    let resource = match document.get("resource").and_then(Value::as_str) {
        Some(resource) if !resource.is_empty() => resource,
        _ => {
            return Err(invalid_metadata(
                status,
                "resource metadata has no resource",
            ));
        }
    };
    if resource != bind_resource {
        return Err(invalid_metadata(
            status,
            &format!(
                "resource metadata resource {} does not equal the resource it was read for, {}",
                origin_of(resource),
                origin_of(bind_resource)
            ),
        ));
    }
    let authorization_servers = match document.get("authorization_servers") {
        None => None,
        Some(value) if is_string_array(value) => serde_json::from_value(value.clone()).ok(),
        Some(_) => {
            return Err(invalid_metadata(
                status,
                "resource metadata authorization_servers must be an array of strings",
            ));
        }
    };
    Ok(ProtectedResourceMetadata {
        resource: resource.to_string(),
        authorization_servers,
    })
}

fn parse_object(
    body: &[u8],
    status: StatusCode,
    what: &str,
) -> Result<serde_json::Map<String, Value>, Error> {
    match serde_json::from_slice::<Value>(body) {
        Ok(Value::Object(document)) => Ok(document),
        Ok(_) => Err(invalid_metadata(
            status,
            &format!("{what} is not a JSON object"),
        )),
        Err(_) => Err(invalid_metadata(status, &format!("{what} is not JSON"))),
    }
}

fn is_string_array(value: &Value) -> bool {
    value
        .as_array()
        .is_some_and(|items| items.iter().all(Value::is_string))
}

#[cfg(all(test, feature = "reqwest"))]
mod tests {
    use super::*;

    #[test]
    fn origin_roots_are_normalized() {
        for (raw, want) in [
            ("https://api.example.com", "https://api.example.com"),
            ("https://api.example.com/", "https://api.example.com"),
            (
                "https://api.example.com:8443",
                "https://api.example.com:8443",
            ),
            ("https://api.example.com:443", "https://api.example.com"),
            ("https://api.example.com:000443", "https://api.example.com"),
            ("https://api.example.com:0080", "https://api.example.com:80"),
            ("HTTPS://API.example.com", "https://api.example.com"),
            ("http://localhost:3000", "http://localhost:3000"),
            ("http://[::1]:3000", "http://[::1]:3000"),
            ("http://127.0.0.1:9999", "http://127.0.0.1:9999"),
        ] {
            assert_eq!(require_origin_root(raw).unwrap(), want, "{raw}");
        }
    }

    #[test]
    fn origin_roots_are_refused() {
        for raw in [
            "http://api.example.com",
            "https://api.example.com:notaport",
            "https://h:99999",
            "https://h:0",
            "https://h:+1",
            "https://h:",
            "http://[::1]:notaport",
            "http://[::1]:",
            "https://api.example.com/tenant/1",
            "https://api.example.com/a/..",
            "https://api.example.com?x=1",
            "https://api.example.com?",
            "https://api.example.com#frag",
            "https://api.example.com#",
            "https://user:pass@api.example.com",
            "https://@api.example.com",
            "https:api.example.com",
            "https:\\\\api.example.com",
            "https://api.example.com\n",
            "https:///",
            "ftp://api.example.com",
            "not a url",
            "",
        ] {
            let error = require_origin_root(raw).unwrap_err();
            assert_eq!(error.code(), ErrorCode::Usage, "{raw:?}");
        }
    }

    #[test]
    fn a_refused_origin_is_named_by_its_origin_only() {
        let error = require_origin_root("https://api.example.com/x?token=secret").unwrap_err();
        assert!(!error.message().contains("secret"), "{}", error.message());
    }

    #[test]
    fn server_metadata_keeps_absent_endpoints_absent() {
        let body = br#"{"issuer":"https://as.example","token_endpoint":"https://as.example/t"}"#;
        let metadata = parse_server_metadata(body, "https://as.example", StatusCode::OK)
            .map_err(MetadataFailure::into_error)
            .unwrap();
        assert_eq!(metadata.authorization_endpoint, None);
        assert_eq!(metadata.grant_types_supported, None);
        assert_eq!(metadata.token_endpoint, "https://as.example/t");
    }

    #[test]
    fn server_metadata_refuses_a_null_list() {
        let body =
            br#"{"issuer":"https://as.example","token_endpoint":"https://as.example/t","scopes_supported":null}"#;
        let error = parse_server_metadata(body, "https://as.example", StatusCode::OK)
            .map_err(MetadataFailure::into_error)
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::ApiError);
        assert_eq!(error.http_status(), Some(200));
    }

    #[test]
    fn a_selection_error_is_read_back_from_the_source_chain() {
        let cause = Error::new(ErrorCode::ApiError, "boom")
            .with_status(503)
            .retryable(true);
        let error = SelectionError::into_error(
            SelectionFailure::AsFetchFailed,
            "fetch failed".to_string(),
            Some(cause),
        );
        assert_eq!(error.code(), ErrorCode::ApiError);
        assert_eq!(error.http_status(), Some(503));
        assert!(error.is_retryable());
        let selection = SelectionError::of(&error).unwrap();
        assert_eq!(selection.reason(), SelectionFailure::AsFetchFailed);
        assert_eq!(selection.cause().unwrap().message(), "boom");
        assert!(SelectionError::of(&Error::usage("plain")).is_none());
    }
}

/// The data-only scenarios of `conformance/oauth/fixtures`, each driven against wiremock
/// origins substituted for the placeholders (see `conformance/oauth/README.md`).
#[cfg(all(test, feature = "reqwest"))]
mod conformance {
    use std::net::TcpListener;
    use std::path::PathBuf;

    use serde::Deserialize;
    use serde_json::{Value, json};
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    use super::*;

    #[derive(Deserialize)]
    #[serde(rename_all = "camelCase")]
    struct Exchange {
        origin: Option<String>,
        status: Option<u16>,
        #[serde(default)]
        transport_error: bool,
        body: Option<Value>,
        #[serde(default)]
        oversized: bool,
        redirect_to: Option<String>,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "camelCase")]
    struct Expect {
        outcome: String,
        selected_issuer: Option<String>,
        fallback_reason: Option<String>,
        error: Option<String>,
        launchpad_contacted: Option<bool>,
        error_category: Option<String>,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "camelCase")]
    struct Fixture {
        name: String,
        operation: String,
        resource_origin: Option<String>,
        issuer_origin: Option<String>,
        expected_issuer: Option<String>,
        hop1: Option<Exchange>,
        hop2: Option<Exchange>,
        expect: Expect,
    }

    fn fixture_dir() -> PathBuf {
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../conformance/oauth/fixtures")
    }

    fn fixture_paths() -> Vec<PathBuf> {
        let mut paths: Vec<PathBuf> = std::fs::read_dir(fixture_dir())
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

    /// An origin nothing listens on: bound, read, and released.
    fn dead_origin() -> String {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        drop(listener);
        format!("http://{address}")
    }

    fn template(exchange: &Exchange) -> ResponseTemplate {
        if exchange.oversized {
            return ResponseTemplate::new(200)
                .insert_header("content-type", "application/json")
                .set_body_bytes(vec![b'x'; super::super::transport::MAX_RESPONSE_BYTES + 1]);
        }
        if let Some(location) = &exchange.redirect_to {
            return ResponseTemplate::new(exchange.status.unwrap_or(302))
                .insert_header("location", location.as_str());
        }
        let template = ResponseTemplate::new(exchange.status.unwrap_or(200));
        match &exchange.body {
            Some(body) => template.set_body_json(body),
            None => template,
        }
    }

    async fn mount(server: &MockServer, well_known: &str, exchange: &Exchange) {
        Mock::given(method("GET"))
            .and(path(well_known))
            .respond_with(template(exchange))
            .mount(server)
            .await;
    }

    async fn requests_seen(server: &MockServer) -> usize {
        server.received_requests().await.unwrap_or_default().len()
    }

    fn category(name: &str) -> ErrorCode {
        match name {
            "usage" => ErrorCode::Usage,
            "validation" => ErrorCode::Validation,
            "api_error" => ErrorCode::ApiError,
            "network" => ErrorCode::Network,
            "auth_required" => ErrorCode::AuthRequired,
            other => panic!("unknown error category {other}"),
        }
    }

    fn failure(name: &str) -> Option<SelectionFailure> {
        Some(match name {
            "ambiguous_issuers" => SelectionFailure::AmbiguousIssuers,
            "expected_issuer_unavailable" => SelectionFailure::ExpectedIssuerUnavailable,
            "invalid_issuer_origin" => SelectionFailure::InvalidIssuerOrigin,
            "as_fetch_failed" => SelectionFailure::AsFetchFailed,
            "issuer_mismatch" => SelectionFailure::IssuerMismatch,
            "capability_unavailable" => SelectionFailure::CapabilityUnavailable,
            _ => return None,
        })
    }

    struct Scenario {
        fixture: Fixture,
        client: OAuthClient,
        launchpad: MockServer,
        result: Result<Option<DiscoveryOutcome>, Error>,
    }

    /// Stages one fixture against fresh role servers and drives its operation.
    async fn run(fixture_path: &PathBuf) -> Option<Scenario> {
        let raw = std::fs::read_to_string(fixture_path).unwrap();
        let transport_failure = raw.contains("\"transportError\": true");

        let resource = MockServer::start().await;
        let bc5 = MockServer::start().await;
        let issuer = MockServer::start().await;
        let launchpad = MockServer::start().await;
        let resource_origin = if transport_failure {
            dead_origin()
        } else {
            resource.uri()
        };

        let substituted = raw
            .replace("{{RESOURCE_ORIGIN}}", &resource_origin)
            .replace("{{BC5_ISSUER}}", &bc5.uri())
            .replace("{{ISSUER_ORIGIN}}", &issuer.uri())
            .replace("{{LAUNCHPAD_ORIGIN}}", &launchpad.uri());
        let fixture: Fixture = serde_json::from_str(&substituted).unwrap();
        let name = fixture.name.as_str();

        Mock::given(method("GET"))
            .and(path(WELL_KNOWN_AS))
            .respond_with(ResponseTemplate::new(200).set_body_json(json!({
                "issuer": launchpad.uri(),
                "authorization_endpoint": format!("{}/authorization/new", launchpad.uri()),
                "token_endpoint": format!("{}/authorization/token", launchpad.uri()),
            })))
            .mount(&launchpad)
            .await;

        // wiremock listens on IPv4 loopback, so the bracketed-IPv6 origin is verified at the
        // parser boundary — the fixture's point is that a transport parser accepts it.
        if let Some(origin) = fixture
            .resource_origin
            .as_deref()
            .filter(|origin| origin.contains('['))
            && fixture.expect.outcome == "selected"
        {
            assert_eq!(
                require_origin_root(origin).unwrap(),
                fixture.expect.selected_issuer.as_deref().unwrap(),
                "{name}"
            );
            return None;
        }

        if let Some(hop1) = &fixture.hop1
            && !hop1.transport_error
        {
            mount(&resource, WELL_KNOWN_RESOURCE, hop1).await;
        }
        if let Some(hop2) = &fixture.hop2 {
            let server = match hop2.origin.as_deref() {
                Some(origin) if origin == bc5.uri() => &bc5,
                Some(origin) if origin == issuer.uri() => &issuer,
                other => panic!("{name}: hop2 origin {other:?} matches no role server"),
            };
            mount(server, WELL_KNOWN_AS, hop2).await;
        }

        let client = OAuthClient::shipped()
            .unwrap()
            .with_launchpad_issuer(launchpad.uri());
        let result = match fixture.operation.as_str() {
            "discoverFromResource" => client
                .discover_from_resource(
                    fixture.resource_origin.as_deref().unwrap(),
                    fixture.expected_issuer.as_deref(),
                )
                .await
                .map(Some),
            "discoverProtectedResource" => client
                .discover_protected_resource(fixture.resource_origin.as_deref().unwrap())
                .await
                .map(|metadata| {
                    if let Some(want) = &fixture.expect.selected_issuer {
                        assert_eq!(&metadata.resource, want, "{name}");
                    }
                    None
                }),
            "discover" => client
                .discover(fixture.issuer_origin.as_deref().unwrap())
                .await
                .map(|metadata| Some(DiscoveryOutcome::Selected(metadata))),
            other => panic!("{name}: unknown operation {other}"),
        };
        Some(Scenario {
            fixture,
            client,
            launchpad,
            result,
        })
    }

    /// Holds the scenario's outcome against its `expect` block.
    async fn check(scenario: Scenario) {
        let Scenario {
            fixture,
            client,
            launchpad,
            result,
        } = scenario;
        let name = fixture.name.as_str();
        match fixture.expect.outcome.as_str() {
            "raise" => {
                let error = result
                    .err()
                    .unwrap_or_else(|| panic!("{name}: expected an error"));
                if let Some(category_name) = &fixture.expect.error_category {
                    assert_eq!(error.code(), category(category_name), "{name}: {error:?}");
                }
                match fixture.expect.error.as_deref().unwrap() {
                    "usage" => assert_eq!(error.code(), ErrorCode::Usage, "{name}"),
                    "invalid_metadata" | "api_error" => {
                        assert_eq!(error.code(), ErrorCode::ApiError, "{name}: {error:?}");
                    }
                    typed => {
                        let selection = SelectionError::of(&error)
                            .unwrap_or_else(|| panic!("{name}: no selection failure on {error:?}"));
                        assert_eq!(selection.reason(), failure(typed).unwrap(), "{name}");
                    }
                }
            }
            "fallback" => {
                let outcome = result
                    .unwrap_or_else(|error| panic!("{name}: expected fallback, got {error:?}"))
                    .unwrap();
                assert_eq!(
                    outcome.fallback().map(|reason| reason.as_str()),
                    fixture.expect.fallback_reason.as_deref(),
                    "{name}"
                );
                if fixture.expect.launchpad_contacted == Some(true) {
                    client.discover_launchpad().await.unwrap();
                    assert!(
                        requests_seen(&launchpad).await > 0,
                        "{name}: Launchpad not contacted"
                    );
                }
            }
            "selected" => {
                let outcome = result
                    .unwrap_or_else(|error| panic!("{name}: expected selected, got {error:?}"));
                if let Some(outcome) = outcome {
                    let metadata = outcome
                        .selected()
                        .unwrap_or_else(|| panic!("{name}: fell back: {outcome:?}"));
                    if let Some(want) = &fixture.expect.selected_issuer {
                        assert_eq!(&metadata.issuer, want, "{name}");
                    }
                }
            }
            other => panic!("{name}: unknown outcome {other}"),
        }

        if fixture.expect.launchpad_contacted == Some(false) {
            assert_eq!(
                requests_seen(&launchpad).await,
                0,
                "{name}: Launchpad was contacted"
            );
        }
    }

    #[tokio::test]
    async fn every_discovery_fixture_holds() {
        for path in fixture_paths() {
            if let Some(scenario) = run(&path).await {
                check(scenario).await;
            }
        }
    }
}
