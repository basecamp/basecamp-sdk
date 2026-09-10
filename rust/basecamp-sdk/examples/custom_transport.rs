//! Bringing your own HTTP client and observing every operation through `Hooks`.
//!
//! The transport below answers every request itself, so this example runs without a
//! network or a token:
//!
//! ```sh
//! cargo run --example custom_transport
//! ```

use std::time::Duration;

use async_trait::async_trait;
use basecamp_sdk::hooks::{OperationInfo, OperationResult, RequestInfo};
use basecamp_sdk::http::{Body, HttpClient, Request, Response, StatusCode};
use basecamp_sdk::services::projects::ListProjectsParams;
use basecamp_sdk::{Client, Config, Error, Hooks};
use bytes::Bytes;

/// A transport that answers one canned page for every request.
struct CannedTransport;

#[async_trait]
impl HttpClient for CannedTransport {
    async fn send(&self, request: Request<Bytes>) -> Result<Response<Body>, Error> {
        println!("-> {} {}", request.method(), request.uri());
        let body = Bytes::from_static(br#"[{"id": 1, "name": "Launch", "status": "active"}]"#);
        let mut response = Response::new(Body::from(body));
        *response.status_mut() = StatusCode::OK;
        response.headers_mut().insert(
            "content-type",
            "application/json".parse().expect("static header"),
        );
        Ok(response)
    }
}

/// Hooks see every operation, attempt and retry; nothing they receive carries a credential.
struct Log;

impl Hooks for Log {
    fn on_operation_start(&self, info: &OperationInfo) {
        println!("operation {} on {}", info.operation, info.service);
    }

    fn on_operation_end(&self, info: &OperationInfo, result: &OperationResult<'_>) {
        println!("operation {} took {:?}", info.operation, result.duration);
    }

    fn on_retry(&self, info: &RequestInfo, next_attempt: u32, error: &Error, delay: Duration) {
        println!(
            "retrying {} (attempt {next_attempt}) after {delay:?}: {error}",
            info.url
        );
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::builder(Config::default())
        .access_token("example-token")
        .http_client(CannedTransport)
        .hooks(Log)
        .build()?;
    let account = client.for_account("999");

    let page = account
        .projects()
        .list(&ListProjectsParams::default())
        .await?;
    println!("{} project(s)", page.len());
    Ok(())
}
