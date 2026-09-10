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
    /// A client that gives an answer `timeout` to arrive.
    pub fn with_timeout(timeout: Duration) -> Result<ReqwestClient, Error> {
        ReqwestClient::from_builder(reqwest::Client::builder().timeout(timeout))
    }

    /// A client built from settings of the caller's own — a proxy, a root certificate, a set
    /// of default headers. Whatever redirect policy the builder carries is replaced with
    /// none, since following one here would hide it from the SDK.
    pub fn from_builder(builder: reqwest::ClientBuilder) -> Result<ReqwestClient, Error> {
        let http = builder
            .redirect(Policy::none())
            .build()
            .map_err(|error| Error::usage(format!("HTTP client: {error}")))?;
        Ok(ReqwestClient { http })
    }
}

impl Default for ReqwestClient {
    fn default() -> ReqwestClient {
        match ReqwestClient::with_timeout(DEFAULT_TIMEOUT) {
            Ok(client) => client,
            Err(error) => panic!("reqwest could not build a client from its defaults: {error}"),
        }
    }
}

#[async_trait]
impl HttpClient for ReqwestClient {
    async fn send(&self, request: Request<Bytes>) -> Result<Response<Body>, Error> {
        let request = reqwest::Request::try_from(request).map_err(Error::network)?;
        let answered = self.http.execute(request).await.map_err(Error::network)?;

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
            Err(error) => Err(Error::network(error)),
        }
    })
}
