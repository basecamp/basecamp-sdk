//! A scripted [`HttpClient`]: answers each request with the case's next mock response, in
//! order, and records what the SDK sent. No socket is involved, which is what lets the
//! `configOverrides.baseUrl` cases run here — the transport records whatever URL the SDK
//! built, so an origin the runner could never dial (`https://3.BasecampAPI.com:443`) is
//! as answerable as the mocked one.
//!
//! Serving semantics are the Go runner's: a `networkError` response is a transport failure
//! after the request is recorded; a response beyond the queue is an empty `200 []` when the
//! case auto-paginates (so a followed `Link` terminates) and a `500` otherwise (so retry
//! exhaustion surfaces the error); a success body that is one object with one array-valued
//! key is unwrapped to the array, the wire shape the SDK decodes.

use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant, SystemTime};

use async_trait::async_trait;
use basecamp_sdk::Error;
use basecamp_sdk::http::{Body, HeaderMap, HttpClient, Request, Response, StatusCode};
use bytes::Bytes;
use serde_json::Value;

use crate::fixtures::MockResponse;
use crate::header_tokens::resolve_header_value;

/// What the transport saw, for the assertions to read afterwards.
#[derive(Debug, Default, Clone)]
pub struct Recorded {
    pub times: Vec<Instant>,
    pub urls: Vec<String>,
    pub paths: Vec<String>,
    pub methods: Vec<String>,
    pub bodies: Vec<Option<Value>>,
    pub headers: Vec<HeaderMap>,
    served: usize,
}

impl Recorded {
    pub fn count(&self) -> usize {
        self.urls.len()
    }
}

#[derive(Clone)]
pub struct ScriptedTransport {
    responses: Vec<MockResponse>,
    auto_paginates: bool,
    recorded: Arc<Mutex<Recorded>>,
}

impl ScriptedTransport {
    pub fn new(responses: Vec<MockResponse>, auto_paginates: bool) -> ScriptedTransport {
        ScriptedTransport {
            responses,
            auto_paginates,
            recorded: Arc::new(Mutex::new(Recorded::default())),
        }
    }

    pub fn recorded(&self) -> Recorded {
        self.recorded
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner)
            .clone()
    }

    fn record(&self, request: &Request<Bytes>) -> usize {
        let mut recorded = self
            .recorded
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner);
        recorded.times.push(Instant::now());
        recorded.urls.push(request.uri().to_string());
        recorded.paths.push(request.uri().path().to_string());
        recorded.methods.push(request.method().to_string());
        recorded.bodies.push(if request.body().is_empty() {
            None
        } else {
            serde_json::from_slice(request.body()).ok()
        });
        recorded.headers.push(request.headers().clone());
        let index = recorded.served;
        recorded.served += 1;
        index
    }
}

#[async_trait]
impl HttpClient for ScriptedTransport {
    async fn send(&self, request: Request<Bytes>) -> Result<Response<Body>, Error> {
        let index = self.record(&request);
        let Some(mock) = self.responses.get(index) else {
            return Ok(if self.auto_paginates {
                json_response(StatusCode::OK, Bytes::from_static(b"[]"))
            } else {
                json_response(
                    StatusCode::INTERNAL_SERVER_ERROR,
                    Bytes::from_static(br#"{"error": "No more mock responses"}"#),
                )
            });
        };
        if mock.delay > 0 {
            tokio::time::sleep(Duration::from_millis(mock.delay)).await;
        }
        if mock.network_error {
            return Err(Error::network(std::io::Error::other(
                "simulated network error",
            )));
        }
        // A header the fixture spells wrong is a fixture defect, not a network condition;
        // surfacing it as a usage error fails the case with the resolver's message.
        serve(mock).map_err(|error| Error::usage(format!("harness: fixture error: {error}")))
    }
}

fn json_response(status: StatusCode, body: Bytes) -> Response<Body> {
    let mut response = Response::new(Body::from(body));
    *response.status_mut() = status;
    response.headers_mut().insert(
        "content-type",
        "application/json".parse().expect("static header"),
    );
    response
}

fn serve(mock: &MockResponse) -> Result<Response<Body>, String> {
    let body = match &mock.body {
        Some(body) => {
            serde_json::to_vec(&normalize_body(body, mock.status)).map_err(|e| e.to_string())?
        }
        None => Vec::new(),
    };
    let status = StatusCode::from_u16(mock.status).map_err(|e| e.to_string())?;
    let mut response = json_response(status, Bytes::from(body));
    // Resolved at serve time: a `{{httpdate+Ns}}` value is relative to NOW, not to when
    // the fixture was loaded.
    for (name, value) in &mock.headers {
        let resolved = resolve_header_value(value, SystemTime::now())?;
        let name: http::HeaderName = name.parse().map_err(|e| format!("header {name:?}: {e}"))?;
        let value: http::HeaderValue = resolved
            .parse()
            .map_err(|e| format!("header {name}: {e}"))?;
        response.headers_mut().insert(name, value);
    }
    Ok(response)
}

/// A success body that is one object with a single array-valued key is served as the array:
/// the fixtures wrap list bodies the way the Smithy model does, and the wire does not.
/// Error bodies are served verbatim — `{"payload_url": ["is invalid"]}` is a field map.
fn normalize_body(body: &Value, status: u16) -> Value {
    if status < 400
        && let Value::Object(fields) = body
        && fields.len() == 1
        && let Some(inner @ Value::Array(_)) = fields.values().next()
    {
        return inner.clone();
    }
    body.clone()
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn a_single_array_key_success_body_is_unwrapped() {
        assert_eq!(
            normalize_body(&json!({"projects": [1, 2]}), 200),
            json!([1, 2])
        );
    }

    #[test]
    fn error_bodies_and_multi_key_bodies_are_served_verbatim() {
        let field_map = json!({"payload_url": ["is invalid"]});
        assert_eq!(normalize_body(&field_map, 422), field_map);
        let two_keys = json!({"a": [1], "b": 2});
        assert_eq!(normalize_body(&two_keys, 200), two_keys);
        let scalar = json!({"id": 1});
        assert_eq!(normalize_body(&scalar, 200), scalar);
    }

    #[tokio::test]
    async fn overrun_is_an_empty_page_when_paginating_and_a_500_otherwise() {
        let paginating = ScriptedTransport::new(vec![], true);
        let response = paginating.send(Request::new(Bytes::new())).await.unwrap();
        assert_eq!(response.status(), StatusCode::OK);

        let plain = ScriptedTransport::new(vec![], false);
        let response = plain.send(Request::new(Bytes::new())).await.unwrap();
        assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
        assert_eq!(plain.recorded().count(), 1);
    }

    #[tokio::test]
    async fn a_network_error_is_recorded_then_fails() {
        let transport = ScriptedTransport::new(
            vec![MockResponse {
                network_error: true,
                ..MockResponse::default()
            }],
            false,
        );
        assert!(transport.send(Request::new(Bytes::new())).await.is_err());
        assert_eq!(transport.recorded().count(), 1);
    }
}
