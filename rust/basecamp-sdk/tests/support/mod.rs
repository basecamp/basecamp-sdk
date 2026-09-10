#![allow(dead_code, unreachable_pub, clippy::unwrap_used, clippy::expect_used)]

use std::sync::{Arc, Mutex};

use std::time::Duration;

use async_trait::async_trait;
use basecamp_sdk::hooks::{Hooks, OperationInfo, OperationResult, RequestInfo, RequestResult};
use basecamp_sdk::http::{Body, HttpClient, Request, Response, StatusCode};
use basecamp_sdk::{AccountClient, Client, Config, Error};
use bytes::Bytes;
use wiremock::MockServer;

pub const PROJECT: &str = r#"{"id": 12345, "name": "Test Project", "status": "active", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z", "url": "https://3.basecampapi.com/999/projects/12345.json", "app_url": "https://3.basecamp.com/999/projects/12345", "dock": [], "bookmarked": false, "purpose": "topic", "clients_enabled": false, "description": ""}"#;

pub fn project(id: i64) -> serde_json::Value {
    let mut value: serde_json::Value = serde_json::from_str(PROJECT).unwrap();
    value["id"] = serde_json::json!(id);
    value["name"] = serde_json::json!(format!("Project {id}"));
    value
}

pub fn account(server: &MockServer) -> AccountClient {
    account_with(server, Config::default())
}

pub fn account_with(server: &MockServer, config: Config) -> AccountClient {
    Client::builder(
        config
            .with_base_url(server.uri())
            .with_timeout(std::time::Duration::from_secs(86_400)),
    )
    .access_token("test-token")
    .build()
    .unwrap()
    .for_account("999")
}

/// An [`HttpClient`] that answers from a script and remembers what it was sent.
pub struct Scripted {
    answers: Mutex<Vec<Answer>>,
    pub sent: Mutex<Vec<Request<Bytes>>>,
}

pub enum Answer {
    Status(u16, Vec<(&'static str, &'static str)>, &'static str),
    NetworkError,
    /// The transport's per-attempt timeout elapsed.
    Timeout,
    /// No answer ever comes: what an operation deadline cuts short.
    Hang,
    /// A status and headers arrive, then the connection breaks while the body streams.
    BrokenBody(u16),
}

impl Scripted {
    pub fn new(answers: Vec<Answer>) -> Arc<Scripted> {
        Arc::new(Scripted {
            answers: Mutex::new(answers),
            sent: Mutex::new(Vec::new()),
        })
    }

    pub fn sent_count(&self) -> usize {
        self.sent.lock().unwrap().len()
    }
}

#[async_trait]
impl HttpClient for Scripted {
    async fn send(&self, request: Request<Bytes>) -> Result<Response<Body>, Error> {
        let (method, uri, headers, body) = (
            request.method().clone(),
            request.uri().clone(),
            request.headers().clone(),
            request.body().clone(),
        );
        let mut copy = Request::builder()
            .method(method)
            .uri(uri)
            .body(body)
            .unwrap();
        *copy.headers_mut() = headers;
        self.sent.lock().unwrap().push(copy);
        let answer = {
            let mut answers = self.answers.lock().unwrap();
            assert!(!answers.is_empty(), "no scripted answer left");
            answers.remove(0)
        };
        match answer {
            Answer::NetworkError => Err(Error::network(std::io::Error::other("connection reset"))),
            Answer::Timeout => Err(Error::network_timeout(std::io::Error::new(
                std::io::ErrorKind::TimedOut,
                "operation timed out",
            ))),
            Answer::Hang => std::future::pending().await,
            Answer::BrokenBody(status) => {
                let chunks = futures_util::stream::iter([
                    Ok(Bytes::from_static(b"{\"id\": 1")),
                    Err(Error::network(std::io::Error::other("connection reset"))),
                ]);
                let mut response = Response::new(Body::from_stream(chunks, None));
                *response.status_mut() = StatusCode::from_u16(status).unwrap();
                Ok(response)
            }
            Answer::Status(status, headers, body) => {
                let mut response = Response::new(Body::from(body));
                *response.status_mut() = StatusCode::from_u16(status).unwrap();
                for (name, value) in headers {
                    response.headers_mut().insert(name, value.parse().unwrap());
                }
                Ok(response)
            }
        }
    }
}

pub fn scripted_account(script: Arc<Scripted>, config: Config) -> AccountClient {
    Client::builder(
        config
            .with_base_url("https://3.basecampapi.com")
            .with_timeout(std::time::Duration::from_secs(86_400)),
    )
    .access_token("test-token")
    .http_client(script)
    .build()
    .unwrap()
    .for_account("999")
}

/// Hooks that write every callback down, in order.
#[derive(Default)]
pub struct HookLog(pub Mutex<Vec<String>>);

impl HookLog {
    pub fn lines(&self) -> Vec<String> {
        self.0.lock().unwrap().clone()
    }

    fn push(&self, line: String) {
        self.0.lock().unwrap().push(line);
    }
}

impl Hooks for HookLog {
    fn on_operation_start(&self, info: &OperationInfo) {
        self.push(format!(
            "op start {} project={:?} resource={:?}",
            info.operation, info.project_id, info.resource_id
        ));
    }

    fn on_operation_end(&self, info: &OperationInfo, result: &OperationResult<'_>) {
        self.push(format!(
            "op end {} ok={}",
            info.operation,
            result.error.is_none()
        ));
    }

    fn on_request_start(&self, info: &RequestInfo) {
        self.push(format!("req start {}", info.attempt));
    }

    fn on_request_end(&self, info: &RequestInfo, result: &RequestResult<'_>) {
        self.push(format!(
            "req end {} {:?} {}",
            info.attempt,
            result.status.map(|s| s.as_u16()),
            result
                .error
                .map_or("ok".to_string(), |e| e.code().to_string())
        ));
    }

    fn on_retry(&self, info: &RequestInfo, next: u32, _error: &Error, delay: Duration) {
        self.push(format!("retry {} -> {next} in {delay:?}", info.attempt));
    }
}
