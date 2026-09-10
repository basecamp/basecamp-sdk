//! Bringing your own transport and watching every request through hooks.
//!
//! The SDK sends on one [`HttpClient`]; this example wraps the shipped reqwest client to
//! count requests, and installs [`Hooks`] that print each attempt.

use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

use async_trait::async_trait;
use basecamp_sdk::hooks::{Hooks, RequestInfo, RequestResult};
use basecamp_sdk::http::{Body, HttpClient, Request, ReqwestClient, Response};
use basecamp_sdk::{Client, Config, Error};
use bytes::Bytes;

struct Counting {
    inner: ReqwestClient,
    sent: AtomicUsize,
}

#[async_trait]
impl HttpClient for Counting {
    async fn send(&self, request: Request<Bytes>) -> Result<Response<Body>, Error> {
        self.sent.fetch_add(1, Ordering::Relaxed);
        self.inner.send(request).await
    }
}

struct Printing;

impl Hooks for Printing {
    fn on_request_start(&self, info: &RequestInfo) {
        println!("→ {} {} (attempt {})", info.method, info.url, info.attempt);
    }

    fn on_request_end(&self, info: &RequestInfo, result: &RequestResult<'_>) {
        println!(
            "← attempt {}: {:?} in {:?}",
            info.attempt, result.status, result.duration
        );
    }

    fn on_retry(&self, _info: &RequestInfo, next: u32, error: &Error, delay: Duration) {
        println!("  retrying as attempt {next} in {delay:?}: {error}");
    }
}

#[tokio::main]
async fn main() -> Result<(), Error> {
    let token = std::env::var("BASECAMP_TOKEN").expect("BASECAMP_TOKEN");
    let account_id = std::env::var("BASECAMP_ACCOUNT").expect("BASECAMP_ACCOUNT");

    let transport = Arc::new(Counting {
        inner: ReqwestClient::with_timeout(Duration::from_secs(10))?,
        sent: AtomicUsize::new(0),
    });
    let account = Client::builder(Config::default())
        .access_token(token)
        .http_client(transport.clone())
        .hooks(Printing)
        .operation_deadline(Duration::from_secs(60))
        .build()?
        .for_account(account_id);

    let me = account.people().me().await?;
    println!("Hello, {}", me.name.expose());
    println!("{} request(s) sent", transport.sent.load(Ordering::Relaxed));
    Ok(())
}
