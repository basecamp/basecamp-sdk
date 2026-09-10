//! SPEC §14 wire-level cases, from `conformance/tests/downloads.json`.

mod support;

use std::time::Duration;

use basecamp_sdk::{Config, ErrorCode};
use support::{Answer, Scripted, account_with, scripted_account};
use wiremock::matchers::{header_exists, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn no_jitter() -> Config {
    let mut config = Config::default();
    config.max_jitter = Duration::ZERO;
    config
}

fn blob_url(server: &MockServer) -> String {
    format!(
        "{}/999999999/blobs/abcd1234/download/logo.png",
        server.uri()
    )
}

#[tokio::test]
async fn hop_one_is_authenticated_and_hop_two_is_bare() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999999999/blobs/abcd1234/download/logo.png"))
        .and(header_exists("Authorization"))
        .respond_with(ResponseTemplate::new(302).insert_header("Location", "/signed/logo.png"))
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/signed/logo.png"))
        .respond_with(ResponseTemplate::new(200).set_body_raw("pixels", "image/png"))
        .expect(1)
        .mount(&server)
        .await;
    let result = account_with(&server, no_jitter())
        .download_url(&format!(
            "https://storage.example.com{}",
            "/999999999/blobs/abcd1234/download/logo.png"
        ))
        .await
        .unwrap();
    assert_eq!(result.body, "pixels");
    assert_eq!(result.content_type, "image/png");
    assert_eq!(result.filename, "logo.png");
    assert_eq!(result.content_length, 6);
    let requests = server.received_requests().await.unwrap();
    assert!(requests[0].headers.get("authorization").is_some());
    assert!(requests[0].headers.get("accept").is_none());
    assert!(requests[1].headers.get("authorization").is_none());
    assert_eq!(requests[1].url.path(), "/signed/logo.png");
}

#[tokio::test]
async fn a_direct_2xx_is_the_download() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999999999/blobs/abcd1234/download/doc.pdf"))
        .respond_with(ResponseTemplate::new(200).set_body_raw("pdf-data", "application/pdf"))
        .expect(1)
        .mount(&server)
        .await;
    let result = account_with(&server, no_jitter())
        .download_url(&format!(
            "{}/999999999/blobs/abcd1234/download/doc.pdf",
            server.uri()
        ))
        .await
        .unwrap();
    assert_eq!(result.body, "pdf-data");
}

#[tokio::test(start_paused = true)]
async fn hop_one_retries_on_503_and_network_errors_but_not_500() {
    let script = Scripted::new(vec![
        Answer::Status(503, vec![], ""),
        Answer::NetworkError,
        Answer::Status(
            302,
            vec![("location", "https://3.basecampapi.com/signed/logo.png")],
            "",
        ),
        Answer::Status(200, vec![("content-type", "image/png")], "pixels"),
    ]);
    let client = scripted_account(script.clone(), no_jitter());
    let started = tokio::time::Instant::now();
    let result = client
        .download_url("https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png")
        .await
        .unwrap();
    assert_eq!(result.body, "pixels");
    assert_eq!(script.sent_count(), 4);
    assert!(started.elapsed() >= Duration::from_secs(3), "1s then 2s");
    let sent = script.sent.lock().unwrap();
    assert!(sent[2].headers().get("authorization").is_some());
    assert!(sent[3].headers().get("authorization").is_none());

    let script = Scripted::new(vec![Answer::Status(
        500,
        vec![],
        r#"{"error": "Internal server error"}"#,
    )]);
    let error = scripted_account(script.clone(), no_jitter())
        .download_url("https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png")
        .await
        .unwrap_err();
    assert_eq!(error.http_status(), Some(500));
    assert_eq!(script.sent_count(), 1);
}

#[tokio::test(start_paused = true)]
async fn hop_one_honours_retry_after_and_a_zero_cap_sends_once() {
    let script = Scripted::new(vec![
        Answer::Status(429, vec![("retry-after", "1")], ""),
        Answer::Status(302, vec![("location", "/signed/logo.png")], ""),
        Answer::Status(200, vec![("content-type", "image/png")], "pixels"),
    ]);
    let started = tokio::time::Instant::now();
    scripted_account(script.clone(), no_jitter())
        .download_url("https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png")
        .await
        .unwrap();
    assert_eq!(script.sent_count(), 3);
    assert!(started.elapsed() >= Duration::from_secs(1));

    let script = Scripted::new(vec![Answer::Status(
        503,
        vec![],
        r#"{"error": "Service Unavailable"}"#,
    )]);
    let error = scripted_account(script.clone(), no_jitter().with_max_retries(0))
        .download_url("https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png")
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::ApiError);
    assert_eq!(script.sent_count(), 1);
}

#[tokio::test]
async fn redirects_without_location_and_on_hop_two_are_refused() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999999999/blobs/abcd1234/download/logo.png"))
        .respond_with(ResponseTemplate::new(302))
        .expect(1)
        .mount(&server)
        .await;
    let error = account_with(&server, no_jitter())
        .download_url(&blob_url(&server))
        .await
        .unwrap_err();
    assert!(
        error.message().contains("no Location"),
        "{}",
        error.message()
    );

    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999999999/blobs/abcd1234/download/logo.png"))
        .respond_with(ResponseTemplate::new(302).insert_header("Location", "/signed/logo.png"))
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/signed/logo.png"))
        .respond_with(ResponseTemplate::new(302).insert_header("Location", "/elsewhere/logo.png"))
        .expect(1)
        .mount(&server)
        .await;
    let error = account_with(&server, no_jitter())
        .download_url(&blob_url(&server))
        .await
        .unwrap_err();
    assert_eq!(error.http_status(), Some(302));
    assert!(
        error.message().contains("not followed"),
        "{}",
        error.message()
    );
    assert!(
        !error.message().contains("/signed"),
        "the signed URL is never rendered"
    );
}
