//! SPEC §14: the two-hop download. An authenticated GET on the API origin answers a
//! redirect to a signed storage URL, which is then fetched bare.

use std::time::{Duration, Instant};

use bytes::Bytes;
use url::Url;

use crate::client::{AccountClient, header_value};
use crate::deadline::Deadline;
use crate::error::{Error, ErrorCode, parse_retry_after_header};
use crate::hooks::{RequestInfo, RequestResult};
use crate::http::header::USER_AGENT;
use crate::http::{Body, Request, Response};
use crate::retry::{
    DEFAULT_RETRY_CONFIG, DOWNLOAD_RETRY_ON, backoff_with_jitter, effective_attempts,
};
use crate::security::{origin_of, redact_url, require_secure_endpoint};
use crate::types::AuthRoutableUrl;

/// What a download answered.
#[derive(Debug, Clone)]
#[non_exhaustive]
pub struct DownloadResult {
    /// The file content.
    pub body: Bytes,
    /// The `Content-Type` the answering host sent.
    pub content_type: String,
    /// The size in bytes, or -1 when unknown.
    pub content_length: i64,
    /// The last segment of the download URL's path as it was given, percent-decoded, or
    /// `download` when it has none. The signed URL hop 1 redirects to names nothing.
    pub filename: String,
}

const REDIRECTS: &[u16] = &[301, 302, 303, 307, 308];

/// What hop 1 ended with: the file itself, or the redirect hop 2 follows.
enum HopOne {
    Downloaded(DownloadResult),
    Redirected(Response<Body>),
}

impl AccountClient {
    /// Downloads an `x-basecamp-auth-routable-url` — an upload's `download_url`, an
    /// attachment's — through the two-hop flow.
    pub async fn download(&self, url: &AuthRoutableUrl) -> Result<DownloadResult, Error> {
        self.download_url(url.as_str()).await
    }

    /// SPEC §14's `downloadURL`: the URL's origin is rewritten to the API origin; hop 1 is an
    /// authenticated GET that never follows redirects, retried on network errors and
    /// `{429, 502, 503, 504}` with `Retry-After` honoured; hop 2 fetches the `Location`
    /// bare — no credentials, no retry, no redirect. Hop-2 errors name only the storage
    /// origin: the signed URL is itself a credential (SPEC §9).
    pub async fn download_url(&self, raw_url: &str) -> Result<DownloadResult, Error> {
        let deadline = self.deadline();
        self.download_within(raw_url, &deadline).await
    }

    async fn download_within(
        &self,
        raw_url: &str,
        deadline: &Deadline,
    ) -> Result<DownloadResult, Error> {
        let given = Url::parse(raw_url)
            .map_err(|_| Error::usage("download URL is not an absolute http(s) URL"))?;
        if !matches!(given.scheme(), "http" | "https") {
            return Err(Error::usage(format!(
                "download URL must be http(s): {}",
                redact_url(&given)
            )));
        }
        let mut url = self.base_url().clone();
        url.set_path(given.path());
        url.set_query(given.query());
        url.set_fragment(None);
        let filename = filename_of(&given);

        let response = match self.download_hop_one(&url, &filename, deadline).await? {
            HopOne::Downloaded(result) => return Ok(result),
            HopOne::Redirected(response) => response,
        };
        let status = response.status();
        if REDIRECTS.contains(&status.as_u16()) {
            let location = response
                .headers()
                .get("location")
                .and_then(|value| value.to_str().ok())
                .ok_or_else(|| {
                    Error::new(ErrorCode::ApiError, "download redirect carried no Location")
                        .with_status(status.as_u16())
                })?;
            let target = url.join(location).map_err(|_| {
                Error::new(
                    ErrorCode::ApiError,
                    format!(
                        "download redirect Location is not a URL ({})",
                        origin_of(location)
                    ),
                )
                .with_status(status.as_u16())
            })?;
            return self.download_hop_two(&target, filename, deadline).await;
        }
        Err(self.download_failure(response, deadline).await)
    }

    #[allow(clippy::too_many_lines)]
    async fn download_hop_one(
        &self,
        url: &Url,
        filename: &str,
        deadline: &Deadline,
    ) -> Result<HopOne, Error> {
        let config = self.config();
        let attempts = effective_attempts(config.max_retries, u32::MAX);
        let shown = Url::parse(&redact_url(url)).unwrap_or_else(|_| url.clone());
        let hooks = self.shared().hooks.clone();
        let mut attempt = 1;
        let mut refreshed = false;
        loop {
            let generation = self.shared().auth.generation();
            let mut request = Request::builder()
                .method(crate::http::Method::GET)
                .uri(url.as_str())
                .body(Bytes::new())
                .map_err(|error| Error::usage(format!("request could not be built: {error}")))?;
            request
                .headers_mut()
                .insert(USER_AGENT, header_value(&self.shared().user_agent)?);
            deadline
                .bound(self.shared().auth.authenticate(&mut request))
                .await?;
            let info = RequestInfo {
                method: crate::http::Method::GET,
                url: shown.clone(),
                attempt,
            };
            crate::hooks::guarded(|| hooks.on_request_start(&info));
            let started = Instant::now();
            let sent = deadline.bound(self.shared().http.send(request)).await;
            let duration = started.elapsed();

            // What this attempt failed with, if it failed; a status outside the retry set
            // is still a failure to the hooks, it is just not one that is retried. A
            // transport failure is projected to the origin (SPEC §9), keeping only whether
            // it was a timeout or the deadline, which decide what happens next.
            let (status, failure, retry_after) = match &sent {
                Err(error) if error.is_deadline_exceeded() => (
                    None,
                    Some(Error::deadline_exceeded(
                        self.config().operation_deadline.unwrap_or_default(),
                    )),
                    None,
                ),
                Err(error) if error.is_timeout() => (
                    None,
                    Some(Error::network_at(&origin_of(url.as_str())).timed_out()),
                    None,
                ),
                Err(_) => (
                    None,
                    Some(Error::network_at(&origin_of(url.as_str()))),
                    None,
                ),
                Ok(response)
                    if response.status().is_success()
                        || REDIRECTS.contains(&response.status().as_u16()) =>
                {
                    (Some(response.status()), None, None)
                }
                Ok(response) => (
                    Some(response.status()),
                    Some(Error::from_response(
                        response.status(),
                        response.headers(),
                        &[],
                    )),
                    parse_retry_after_header(response.headers(), chrono::Utc::now()),
                ),
            };
            let ended = |error: Option<&Error>| {
                crate::hooks::guarded(|| {
                    hooks.on_request_end(
                        &info,
                        &RequestResult {
                            status,
                            duration,
                            error,
                            retry_after,
                        },
                    );
                });
            };

            let Some(cause) = failure else {
                let response = sent.map_err(|_| Error::network_at(&origin_of(url.as_str())))?;
                if REDIRECTS.contains(&response.status().as_u16()) {
                    ended(None);
                    return Ok(HopOne::Redirected(response));
                }
                // A direct answer's body is read inside the attempt — its end is the
                // attempt's end, as the hooks see it — so a connection that breaks while
                // it streams is retried like one that never answered.
                let read = self
                    .read_download(url, filename.to_string(), response, deadline)
                    .await;
                ended(read.as_ref().err());
                match read {
                    Ok(result) => return Ok(HopOne::Downloaded(result)),
                    Err(error)
                        if attempt < attempts
                            && error.code() == ErrorCode::Network
                            && !error.is_timeout()
                            && !error.is_deadline_exceeded() =>
                    {
                        let delay = backoff_with_jitter(
                            &DEFAULT_RETRY_CONFIG,
                            attempt - 1,
                            config.max_jitter,
                        );
                        crate::hooks::guarded(|| hooks.on_retry(&info, attempt + 1, &error, delay));
                        deadline.wait(delay).await?;
                        attempt += 1;
                        continue;
                    }
                    Err(error) => return Err(error),
                }
            };
            ended(Some(&cause));
            if status == Some(crate::http::StatusCode::UNAUTHORIZED) {
                if !refreshed && attempt < attempts && self.shared().auth.refreshable() {
                    refreshed = true;
                    let renewed = match deadline.bound(self.shared().auth.refresh(generation)).await
                    {
                        Ok(renewed) => renewed,
                        Err(cause) if cause.is_deadline_exceeded() => return Err(cause),
                        Err(cause) => {
                            return Err(Error::new(
                                ErrorCode::AuthRequired,
                                "credentials could not be refreshed",
                            )
                            .with_status(401)
                            .with_source(cause));
                        }
                    };
                    if renewed {
                        crate::hooks::guarded(|| {
                            hooks.on_retry(&info, attempt + 1, &cause, Duration::ZERO);
                        });
                        attempt += 1;
                        continue;
                    }
                }
                return Err(self.download_failure(sent?, deadline).await);
            }
            // A timed-out attempt spent its whole per-attempt budget and is not resent
            // (SPEC §14); neither is one the operation deadline cut short.
            let retried = match &sent {
                Err(_) => !cause.is_timeout() && !cause.is_deadline_exceeded(),
                Ok(_) => status.is_some_and(|s| DOWNLOAD_RETRY_ON.contains(&s.as_u16())),
            };
            if retried && attempt < attempts {
                let delay = match retry_after {
                    Some(seconds) => Duration::from_secs(u64::from(seconds)),
                    None => {
                        backoff_with_jitter(&DEFAULT_RETRY_CONFIG, attempt - 1, config.max_jitter)
                    }
                };
                crate::hooks::guarded(|| hooks.on_retry(&info, attempt + 1, &cause, delay));
                deadline.wait(delay).await?;
                attempt += 1;
                continue;
            }
            return match sent {
                Ok(response) => Err(self.download_failure(response, deadline).await),
                Err(_) => Err(cause),
            };
        }
    }

    async fn download_hop_two(
        &self,
        target: &Url,
        filename: String,
        deadline: &Deadline,
    ) -> Result<DownloadResult, Error> {
        let origin = origin_of(target.as_str());
        require_secure_endpoint(target).map_err(|_| {
            Error::usage(format!("download redirect to an insecure origin: {origin}"))
        })?;
        let request = Request::builder()
            .method(crate::http::Method::GET)
            .uri(target.as_str())
            .body(Bytes::new())
            .map_err(|error| Error::usage(format!("request could not be built: {error}")))?;
        let response = deadline
            .bound(self.shared().http.send(request))
            .await
            .map_err(|error| {
                if error.is_deadline_exceeded() {
                    error
                } else {
                    Error::network_at(&origin)
                }
            })?;
        let status = response.status();
        if REDIRECTS.contains(&status.as_u16()) {
            return Err(Error::new(
                ErrorCode::ApiError,
                format!("download redirect from {origin} not followed (HTTP {status})"),
            )
            .with_status(status.as_u16()));
        }
        if !status.is_success() {
            return Err(Error::new(
                ErrorCode::ApiError,
                format!("download failed with status {}", status.as_u16()),
            )
            .with_status(status.as_u16()));
        }
        self.read_download(target, filename, response, deadline)
            .await
    }

    async fn read_download(
        &self,
        url: &Url,
        filename: String,
        response: Response<Body>,
        deadline: &Deadline,
    ) -> Result<DownloadResult, Error> {
        let content_type = response
            .headers()
            .get("content-type")
            .and_then(|value| value.to_str().ok())
            .unwrap_or("application/octet-stream")
            .to_string();
        let declared = response.body().content_length();
        let limit = self.config().max_response_body_bytes;
        let origin = origin_of(url.as_str());
        let body = deadline
            .bound(
                response
                    .into_body()
                    .collect(limit, || Error::response_too_large(limit)),
            )
            .await
            .map_err(|error| {
                // A transport's account of a failing read may render the URL it was
                // reading, which on hop 2 is a credential (SPEC §9): only the origin survives.
                if error.code() == ErrorCode::Network && !error.is_deadline_exceeded() {
                    Error::network_at(&origin)
                } else {
                    error
                }
            })?;
        let content_length = match declared {
            Some(length) => i64::try_from(length).unwrap_or(-1),
            None => i64::try_from(body.len()).unwrap_or(-1),
        };
        Ok(DownloadResult {
            body,
            content_type,
            content_length,
            filename,
        })
    }

    async fn download_failure(&self, response: Response<Body>, deadline: &Deadline) -> Error {
        let status = response.status();
        let headers = response.headers().clone();
        let read = response
            .into_body()
            .collect(crate::error::MAX_ERROR_BODY_BYTES, || {
                Error::response_too_large(crate::error::MAX_ERROR_BODY_BYTES)
            });
        match deadline.bound(read).await {
            Ok(body) => Error::from_response(status, &headers, &body),
            Err(error) if error.is_deadline_exceeded() => error,
            Err(_) => Error::from_response(status, &headers, &[]),
        }
    }
}

/// SPEC §14's `filename`: the last segment of the given URL's path, percent-decoded, or
/// `download` when there is none — the same rule as Go's and TypeScript's.
fn filename_of(url: &Url) -> String {
    let last = url
        .path_segments()
        .and_then(|mut segments| segments.rfind(|segment| !segment.is_empty()))
        .unwrap_or_default();
    if last.is_empty() || last == "." {
        "download".to_string()
    } else {
        percent_encoding::percent_decode_str(last)
            .decode_utf8()
            .map_or_else(|_| last.to_string(), std::borrow::Cow::into_owned)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_filename_is_the_last_segment_decoded_or_download() {
        let named = |raw: &str| filename_of(&Url::parse(raw).unwrap());
        assert_eq!(
            named("https://x/a/report%20final.pdf?sig=1"),
            "report final.pdf"
        );
        assert_eq!(named("https://x/a/logo.png/"), "logo.png");
        assert_eq!(named("https://x/"), "download");
        assert_eq!(named("https://x"), "download");
        assert_eq!(named("https://x/a/%ZZ"), "%ZZ");
    }
}
