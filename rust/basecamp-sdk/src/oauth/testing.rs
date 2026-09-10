//! A scripted [`HttpClient`] for the flows tested in virtual time, where a real socket
//! would race the paused clock.

use std::collections::VecDeque;
use std::sync::Mutex;
use std::time::Duration;

use async_trait::async_trait;
use bytes::Bytes;
use tokio::time::Instant;

use crate::error::Error;
use crate::http::{Body, HeaderName, HeaderValue, HttpClient, Request, Response, StatusCode};

/// What one scripted request is answered with.
pub(super) enum Scripted {
    /// A response.
    Response {
        status: u16,
        headers: Vec<(String, String)>,
        body: String,
    },
    /// No answer, ever: the caller's timeout is what ends it.
    Stall,
    /// A transport failure.
    Fail,
}

impl Scripted {
    pub(super) fn json(status: u16, body: impl serde::Serialize) -> Scripted {
        Scripted::Response {
            status,
            headers: vec![("content-type".to_string(), "application/json".to_string())],
            body: serde_json::to_string(&body).unwrap(),
        }
    }

    pub(super) fn text(status: u16, body: &str) -> Scripted {
        Scripted::Response {
            status,
            headers: Vec::new(),
            body: body.to_string(),
        }
    }

    pub(super) fn header(mut self, name: &str, value: &str) -> Scripted {
        if let Scripted::Response { headers, .. } = &mut self {
            headers.push((name.to_string(), value.to_string()));
        }
        self
    }

    pub(super) fn status(&self) -> u16 {
        match self {
            Scripted::Response { status, .. } => *status,
            Scripted::Stall | Scripted::Fail => 0,
        }
    }
}

/// A request the script saw, and when.
pub(super) struct Seen {
    pub(super) at: Instant,
    pub(super) body: Vec<u8>,
}

/// Answers requests from a queue, in order, and remembers each one.
pub(super) struct Script {
    started: Instant,
    queue: Mutex<VecDeque<Scripted>>,
    seen: Mutex<Vec<Seen>>,
}

impl Script {
    pub(super) fn new(responses: impl IntoIterator<Item = Scripted>) -> std::sync::Arc<Script> {
        std::sync::Arc::new(Script {
            started: Instant::now(),
            queue: Mutex::new(responses.into_iter().collect()),
            seen: Mutex::new(Vec::new()),
        })
    }

    pub(super) fn requests(&self) -> Vec<Seen> {
        std::mem::take(&mut *self.seen.lock().unwrap())
    }

    /// Whole seconds since the script was created, at each request in order.
    pub(super) fn seconds_at_each_request(&self) -> Vec<u64> {
        self.seen
            .lock()
            .unwrap()
            .iter()
            .map(|seen| seen.at.duration_since(self.started).as_secs())
            .collect()
    }
}

#[async_trait]
impl HttpClient for Script {
    async fn send(&self, request: Request<Bytes>) -> Result<Response<Body>, Error> {
        self.seen.lock().unwrap().push(Seen {
            at: Instant::now(),
            body: request.body().to_vec(),
        });
        let next = self.queue.lock().unwrap().pop_front();
        match next {
            Some(Scripted::Response {
                status,
                headers,
                body,
            }) => {
                let mut response = Response::new(Body::from(Bytes::from(body)));
                *response.status_mut() = StatusCode::from_u16(status).unwrap();
                for (name, value) in headers {
                    response.headers_mut().append(
                        HeaderName::from_bytes(name.as_bytes()).unwrap(),
                        HeaderValue::from_str(&value).unwrap(),
                    );
                }
                Ok(response)
            }
            Some(Scripted::Stall) => {
                tokio::time::sleep(Duration::from_secs(1_000_000)).await;
                Err(Error::usage("the stall ended"))
            }
            Some(Scripted::Fail) => Err(Error::network_at("https://as.example")),
            None => panic!("the script has no answer for this request"),
        }
    }
}

/// The form pairs of a request body, in order.
pub(super) fn form_pairs(body: &[u8]) -> Vec<(String, String)> {
    url::form_urlencoded::parse(body)
        .map(|(name, value)| (name.into_owned(), value.into_owned()))
        .collect()
}
