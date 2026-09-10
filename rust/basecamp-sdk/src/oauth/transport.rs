//! The transport policy every OAuth request shares (SPEC §16 "Token-Endpoint Transport
//! Policy" and "SSRF hardening"): a wall-clock bound on the whole request, a body read
//! that stops at its cap, and a redirect refused rather than followed.

use std::time::Duration;

use bytes::Bytes;
use tokio::time::Instant;

use crate::error::{Error, truncate};
use crate::http::header::{ACCEPT, CONTENT_TYPE};
use crate::http::{Body, HttpClient, Method, Request, Response, StatusCode};
use crate::security::origin_of;
use url::Url;

/// How long one OAuth request — a discovery fetch, a token POST, a device-flow POST — is
/// given before it is abandoned.
pub const DEFAULT_REQUEST_TIMEOUT: Duration = Duration::from_secs(30);
/// The most a caller can set the request timeout to; a larger value falls back to the
/// default, so a stalled credential POST is never held open indefinitely.
pub const MAX_REQUEST_TIMEOUT: Duration = Duration::from_secs(3600);
/// The most of a token, device-authorization or discovery body the client reads.
pub(super) const MAX_RESPONSE_BYTES: usize = 1_048_576;
/// `application/x-www-form-urlencoded`, the only body an OAuth endpoint is sent.
pub(super) const FORM_CONTENT_TYPE: &str = "application/x-www-form-urlencoded";

/// A zero timeout or one past the ceiling is the default; anything else is kept.
pub(super) fn normalize_timeout(timeout: Duration) -> Duration {
    if timeout.is_zero() || timeout > MAX_REQUEST_TIMEOUT {
        DEFAULT_REQUEST_TIMEOUT
    } else {
        timeout
    }
}

/// Why a request produced no response.
pub(super) enum TransportFailure {
    /// The deadline passed first.
    TimedOut,
    /// The transport failed to get an answer; the error is the [`HttpClient`]'s own.
    Failed(Error),
}

/// Why a body could not be read whole.
pub(super) enum BodyFailure {
    /// The deadline passed first.
    TimedOut,
    /// The body passed the cap; the error is already typed and carries the status.
    TooLarge(Error),
    /// The stream broke; the error is the [`HttpClient`]'s own.
    Failed(Error),
}

/// Sends `request` and answers with the response before its body is read, or with why
/// there was none by `deadline`.
pub(super) async fn send_within(
    http: &dyn HttpClient,
    deadline: Instant,
    request: Request<Bytes>,
) -> Result<Response<Body>, TransportFailure> {
    match tokio::time::timeout_at(deadline, http.send(request)).await {
        Ok(Ok(response)) => Ok(response),
        Ok(Err(error)) => Err(TransportFailure::Failed(error)),
        Err(_) => Err(TransportFailure::TimedOut),
    }
}

/// Reads `body` whole, up to [`MAX_RESPONSE_BYTES`], by `deadline`.
pub(super) async fn read_within(
    deadline: Instant,
    body: Body,
    status: StatusCode,
) -> Result<Bytes, BodyFailure> {
    let read = body.collect(MAX_RESPONSE_BYTES, || {
        Error::response_too_large(MAX_RESPONSE_BYTES).with_status(status.as_u16())
    });
    match tokio::time::timeout_at(deadline, read).await {
        Ok(Ok(body)) => Ok(body),
        Ok(Err(error)) if error.is_response_too_large() => Err(BodyFailure::TooLarge(error)),
        Ok(Err(error)) => Err(BodyFailure::Failed(error)),
        Err(_) => Err(BodyFailure::TimedOut),
    }
}

/// A form POST to an OAuth endpoint: `Accept: application/json`, the form as its body.
pub(super) fn form_post(url: &Url, form: String) -> Result<Request<Bytes>, Error> {
    Request::builder()
        .method(Method::POST)
        .uri(url.as_str())
        .header(ACCEPT, "application/json")
        .header(CONTENT_TYPE, FORM_CONTENT_TYPE)
        .body(Bytes::from(form))
        .map_err(|_| {
            Error::usage(format!(
                "could not build a request to {}",
                origin_of(url.as_str())
            ))
        })
}

/// A JSON GET of a well-known document.
pub(super) fn json_get(url: &str) -> Result<Request<Bytes>, Error> {
    Request::builder()
        .method(Method::GET)
        .uri(url)
        .header(ACCEPT, "application/json")
        .body(Bytes::new())
        .map_err(|_| Error::usage(format!("could not build a request to {}", origin_of(url))))
}

/// The transport failure an OAuth POST reports: the origin alone, the transport's own
/// account of it discarded, since that account could render the request (SPEC §9).
pub(super) fn network_failure(url: &Url, failure: &TransportFailure) -> Error {
    let error = Error::network_at(&origin_of(url.as_str()));
    match failure {
        TransportFailure::TimedOut => error.with_hint("the request timed out"),
        TransportFailure::Failed(_) => error,
    }
}

/// Whether `status` is a redirect the token endpoint is refused on: 301, 302, 303, 307 or
/// 308. Any other 3xx — 304 above all — is the generic non-2xx failure.
pub(super) fn is_refused_redirect(status: StatusCode) -> bool {
    matches!(status.as_u16(), 301 | 302 | 303 | 307 | 308)
}

/// The RFC 6749 `error` and `error_description` of a failure body, and nothing else of it:
/// the endpoint may echo what it was sent, so the rest is never rendered (SPEC §9).
pub(super) fn oauth_error_fields(body: &[u8]) -> Option<(String, Option<String>)> {
    let value: serde_json::Value = serde_json::from_slice(body).ok()?;
    let code = value.get("error")?.as_str()?;
    if code.is_empty() {
        return None;
    }
    let description = value
        .get("error_description")
        .and_then(serde_json::Value::as_str)
        .filter(|text| !text.is_empty())
        .map(truncate);
    Some((truncate(code), description))
}
