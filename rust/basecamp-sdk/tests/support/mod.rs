#![allow(dead_code, unreachable_pub)]

use std::sync::{Arc, Mutex};

use async_trait::async_trait;
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
        let mut answers = self.answers.lock().unwrap();
        assert!(!answers.is_empty(), "no scripted answer left");
        match answers.remove(0) {
            Answer::NetworkError => Err(Error::network(std::io::Error::other("connection reset"))),
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
