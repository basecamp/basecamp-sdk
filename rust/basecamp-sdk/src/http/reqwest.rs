use std::time::Duration;

use async_trait::async_trait;
use bytes::Bytes;
use futures_util::stream;
use reqwest::redirect::Policy;

use crate::config::DEFAULT_TIMEOUT;
use crate::error::Error;
use crate::http::{Body, HttpClient, Request, Response};

/// The [`HttpClient`] the SDK ships, over [`reqwest`]. This is what a client gets when
/// nothing else is supplied.
///
/// It never follows a redirect, whatever it was built from: the SDK does that itself.
#[derive(Debug, Clone)]
pub struct ReqwestClient {
    http: reqwest::Client,
}

impl ReqwestClient {
    /// A client with the SDK's default per-attempt timeout.
    pub fn new() -> Result<ReqwestClient, Error> {
        ReqwestClient::with_timeout(DEFAULT_TIMEOUT)
    }

    /// A client that gives an answer `timeout` to arrive.
    pub fn with_timeout(timeout: Duration) -> Result<ReqwestClient, Error> {
        ReqwestClient::from_builder(reqwest::Client::builder().timeout(timeout))
    }

    /// A client built from settings of the caller's own — a proxy, a root certificate, a
    /// timeout. Two settings the builder carries are replaced, because each would act
    /// beneath the SDK where it cannot see: redirects are never followed (the SDK follows
    /// its own) and reqwest's own retries are off (SPEC §7's attempt budget counts every
    /// request).
    ///
    /// Default headers are the caller's and cannot be taken back: reqwest adds them to
    /// every request that lacks the header, including a download's second hop, which must
    /// go out bare. A builder carrying a default `Authorization` or `Cookie` would send it
    /// to the storage host, so give it none — the SDK sets every header it needs per
    /// request.
    pub fn from_builder(builder: reqwest::ClientBuilder) -> Result<ReqwestClient, Error> {
        let http = builder
            .redirect(Policy::none())
            .retry(reqwest::retry::never())
            .build()
            .map_err(|error| Error::usage(format!("HTTP client: {error}")))?;
        Ok(ReqwestClient { http })
    }
}

#[async_trait]
impl HttpClient for ReqwestClient {
    async fn send(&self, request: Request<Bytes>) -> Result<Response<Body>, Error> {
        let request = reqwest::Request::try_from(request).map_err(Error::network)?;
        let answered = self.http.execute(request).await.map_err(classify)?;

        let status = answered.status();
        let version = answered.version();
        let headers = answered.headers().clone();
        let content_length = answered.content_length();

        let mut response = Response::new(Body::from_stream(chunks(answered), content_length));
        *response.status_mut() = status;
        *response.version_mut() = version;
        *response.headers_mut() = headers;
        Ok(response)
    }
}

fn chunks(response: reqwest::Response) -> impl stream::Stream<Item = Result<Bytes, Error>> + Send {
    stream::try_unfold(response, |mut response| async move {
        match response.chunk().await {
            Ok(Some(chunk)) => Ok(Some((chunk, response))),
            Ok(None) => Ok(None),
            Err(error) => Err(classify(error)),
        }
    })
}

fn classify(error: reqwest::Error) -> Error {
    if error.is_timeout() {
        Error::network_timeout(error)
    } else {
        Error::network(error)
    }
}
