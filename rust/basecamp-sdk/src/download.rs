//! SPEC §14: the two-hop download. An authenticated GET on the API origin answers a
//! redirect to a signed storage URL, which is then fetched bare.

use std::time::{Duration, Instant};

use bytes::Bytes;
use url::Url;

use crate::client::{AccountClient, header_value};
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
    /// The last segment of the URL's path.
    pub filename: String,
}

const REDIRECTS: &[u16] = &[301, 302, 303, 307, 308];

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
        let given = Url::parse(raw_url)
            .map_err(|_| Error::usage(format!("download URL is not absolute: {raw_url:?}")))?;
        if !matches!(given.scheme(), "http" | "https") {
            return Err(Error::usage(format!(
                "download URL must be http(s): {}",
                redact_url(&given)
            )));
        }
        let mut url = self.base_url().clone();
        url.set_path(given.path());
        url.set_query(given.query());
        url.set_fragment(given.fragment());

        let response = self.download_hop_one(&url).await?;
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
            return self.download_hop_two(&target).await;
        }
        if status.is_success() {
            return self.read_download(&url, response).await;
        }
        Err(self.download_failure(response).await)
    }

    async fn download_hop_one(&self, url: &Url) -> Result<Response<Body>, Error> {
        let config = self.config();
        let attempts = effective_attempts(config.max_retries, u32::MAX);
        let shown = Url::parse(&redact_url(url)).unwrap_or_else(|_| url.clone());
        let hooks = self.shared().hooks.clone();
        let mut attempt = 1;
        loop {
            let mut request = Request::builder()
                .method(crate::http::Method::GET)
                .uri(url.as_str())
                .body(Bytes::new())
                .map_err(|error| Error::usage(format!("request could not be built: {error}")))?;
            request
                .headers_mut()
                .insert(USER_AGENT, header_value(&self.shared().user_agent)?);
            self.shared().auth.authenticate(&mut request).await?;
            let info = RequestInfo {
                method: crate::http::Method::GET,
                url: shown.clone(),
                attempt,
            };
            crate::hooks::guarded(|| hooks.on_request_start(&info));
            let started = Instant::now();
            let sent = self.shared().http.send(request).await;
            let duration = started.elapsed();
            let (failure, retry_after) = match &sent {
                Err(_) => (Some(Error::network_at(&origin_of(url.as_str()))), None),
                Ok(response) if DOWNLOAD_RETRY_ON.contains(&response.status().as_u16()) => (
                    Some(Error::from_response(
                        response.status(),
                        response.headers(),
                        &[],
                    )),
                    parse_retry_after_header(response.headers(), chrono::Utc::now()),
                ),
                Ok(_) => (None, None),
            };
            let status = sent.as_ref().ok().map(Response::status);
            crate::hooks::guarded(|| {
                hooks.on_request_end(
                    &info,
                    &RequestResult {
                        status,
                        duration,
                        error: failure.as_ref(),
                        retry_after,
                    },
                );
            });
            match (sent, failure) {
                (Ok(response), None) => return Ok(response),
                (_, Some(cause)) if attempt < attempts => {
                    let delay = match retry_after {
                        Some(seconds) => Duration::from_secs(u64::from(seconds)),
                        None => backoff_with_jitter(
                            &DEFAULT_RETRY_CONFIG,
                            attempt - 1,
                            config.max_jitter,
                        ),
                    };
                    crate::hooks::guarded(|| hooks.on_retry(&info, attempt + 1, &cause, delay));
                    tokio::time::sleep(delay).await;
                    attempt += 1;
                }
                (Ok(response), Some(_)) => return Err(self.download_failure(response).await),
                (Err(_), _) => return Err(Error::network_at(&origin_of(url.as_str()))),
            }
        }
    }

    async fn download_hop_two(&self, target: &Url) -> Result<DownloadResult, Error> {
        let origin = origin_of(target.as_str());
        require_secure_endpoint(target).map_err(|_| {
            Error::usage(format!("download redirect to an insecure origin: {origin}"))
        })?;
        let request = Request::builder()
            .method(crate::http::Method::GET)
            .uri(target.as_str())
            .body(Bytes::new())
            .map_err(|error| Error::usage(format!("request could not be built: {error}")))?;
        let response = self
            .shared()
            .http
            .send(request)
            .await
            .map_err(|_| Error::network_at(&origin))?;
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
        self.read_download(target, response).await
    }

    async fn read_download(
        &self,
        url: &Url,
        response: Response<Body>,
    ) -> Result<DownloadResult, Error> {
        let content_type = response
            .headers()
            .get("content-type")
            .and_then(|value| value.to_str().ok())
            .unwrap_or("application/octet-stream")
            .to_string();
        let declared = response.body().content_length();
        let limit = self.config().max_response_body_bytes;
        let body = response
            .into_body()
            .collect(limit, || Error::response_too_large(limit))
            .await?;
        let content_length = match declared {
            Some(length) => i64::try_from(length).unwrap_or(-1),
            None => i64::try_from(body.len()).unwrap_or(-1),
        };
        let filename = url
            .path_segments()
            .and_then(|mut segments| segments.next_back())
            .unwrap_or_default()
            .to_string();
        Ok(DownloadResult {
            body,
            content_type,
            content_length,
            filename,
        })
    }

    async fn download_failure(&self, response: Response<Body>) -> Error {
        let status = response.status();
        let headers = response.headers().clone();
        let body = response
            .into_body()
            .collect(crate::error::MAX_ERROR_BODY_BYTES, || {
                Error::response_too_large(crate::error::MAX_ERROR_BODY_BYTES)
            })
            .await
            .unwrap_or_default();
        Error::from_response(status, &headers, &body)
    }
}
