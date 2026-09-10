//! The shipped [`ReqwestClient`] keeps a request's URL out of the error a transport failure
//! becomes. reqwest writes the URL into its own error text, and a signed download or upload
//! URL carries its credential in the query (SPEC §9), so neither the hint nor the source
//! chain may show it.

#![cfg(feature = "reqwest")]
#![allow(clippy::unwrap_used, clippy::expect_used)]

use std::error::Error as _;
use std::time::Duration;

use basecamp_sdk::http::{HttpClient, Request, ReqwestClient};
use basecamp_sdk::{Error, ErrorCode};
use bytes::Bytes;
use tokio::net::TcpListener;

const SIGNATURE: &str = "sig-4f3c2a1b-never-logged";

fn client() -> ReqwestClient {
    ReqwestClient::with_timeout(Duration::from_secs(5)).unwrap()
}

fn signed_request(port: u16) -> Request<Bytes> {
    Request::get(format!(
        "http://127.0.0.1:{port}/storage/blob?signature={SIGNATURE}"
    ))
    .body(Bytes::new())
    .unwrap()
}

/// Every rendering a caller or a log would see: the error itself, its hint and each link
/// of its source chain, by `Display` and by `Debug`.
fn renderings(error: &Error) -> Vec<String> {
    let mut seen = vec![format!("{error}"), format!("{error:?}")];
    seen.extend(error.hint().map(str::to_owned));
    let mut source = error.source();
    while let Some(cause) = source {
        seen.push(format!("{cause}"));
        seen.push(format!("{cause:?}"));
        source = cause.source();
    }
    seen
}

fn assert_signature_withheld(error: &Error) {
    assert_eq!(error.code(), ErrorCode::Network);
    assert!(
        error.source().is_some(),
        "the transport failure is chained as the cause"
    );
    assert!(
        error.hint().is_some_and(|hint| !hint.is_empty()),
        "the failure is still described"
    );
    for text in renderings(error) {
        assert!(
            !text.contains(SIGNATURE),
            "the signed query leaked into {text:?}"
        );
    }
}

#[tokio::test]
async fn a_refused_connection_withholds_the_signed_query() {
    let port = TcpListener::bind("127.0.0.1:0")
        .await
        .unwrap()
        .local_addr()
        .unwrap()
        .port();

    let error = client().send(signed_request(port)).await.err().unwrap();
    assert_signature_withheld(&error);
}
