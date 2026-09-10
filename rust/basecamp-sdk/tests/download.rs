//! SPEC §14 wire-level cases, from `conformance/tests/downloads.json`.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use std::time::Duration;

use basecamp_sdk::{Config, ErrorCode};
use support::{Answer, Scripted, account_with, scripted_account};
use wiremock::matchers::{header_exists, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn no_jitter() -> Config {
    Config {
        max_jitter: Duration::ZERO,
        ..Config::default()
    }
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
    assert_ne!(
        requests[0]
            .headers
            .get("accept")
            .map(|value| value.to_str().unwrap()),
        Some("application/json"),
        "a download asks for bytes, not JSON"
    );
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
    {
        let sent = script.sent.lock().unwrap();
        assert!(sent[2].headers().get("authorization").is_some());
        assert!(sent[3].headers().get("authorization").is_none());
    }

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
        Answer::Status(429, vec![("retry-after", "7")], ""),
        Answer::Status(302, vec![("location", "/signed/logo.png")], ""),
        Answer::Status(200, vec![("content-type", "image/png")], "pixels"),
    ]);
    let started = tokio::time::Instant::now();
    scripted_account(script.clone(), no_jitter())
        .download_url("https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png")
        .await
        .unwrap();
    assert_eq!(script.sent_count(), 3);
    assert_eq!(
        started.elapsed(),
        Duration::from_secs(7),
        "Retry-After replaces the 1 s curve, with nothing added"
    );

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

#[tokio::test]
async fn the_filename_comes_from_the_given_url_not_the_signed_one() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path(
            "/999999999/blobs/abcd1234/download/report%20final.pdf",
        ))
        .respond_with(ResponseTemplate::new(302).insert_header("Location", "/signed/blob?sig=abc"))
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/signed/blob"))
        .respond_with(ResponseTemplate::new(200).set_body_raw("pdf", "application/pdf"))
        .expect(1)
        .mount(&server)
        .await;
    let result = account_with(&server, no_jitter())
        .download_url(&format!(
            "{}/999999999/blobs/abcd1234/download/report%20final.pdf",
            server.uri()
        ))
        .await
        .unwrap();
    assert_eq!(result.filename, "report final.pdf");
}

#[tokio::test(start_paused = true)]
async fn hop_one_does_not_resend_a_timed_out_attempt() {
    let script = Scripted::new(vec![
        Answer::Timeout,
        Answer::Status(200, vec![("content-type", "image/png")], "pixels"),
    ]);
    let error = scripted_account(script.clone(), no_jitter())
        .download_url("https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png")
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Network);
    assert!(error.is_timeout());
    assert!(
        !error.message().contains("abcd1234"),
        "the projection keeps only the origin"
    );
    assert_eq!(script.sent_count(), 1);
}

#[tokio::test(start_paused = true)]
async fn a_deadline_that_cuts_hop_one_short_still_closes_its_hooks() {
    use support::HookLog;
    let script = Scripted::new(vec![Answer::Hang]);
    let log = std::sync::Arc::new(HookLog::default());
    let mut config = no_jitter().with_base_url("https://3.basecampapi.com");
    config.operation_deadline = Some(Duration::from_secs(5));
    let client = basecamp_sdk::Client::builder(config)
        .access_token("t")
        .http_client(script.clone())
        .hooks(log.clone())
        .build()
        .unwrap()
        .for_account("999");
    let error = client
        .download_url("https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png")
        .await
        .unwrap_err();
    assert!(error.is_deadline_exceeded());
    assert_eq!(log.lines(), ["req start 1", "req end 1 None network"]);
}

#[tokio::test(start_paused = true)]
async fn a_direct_download_whose_body_breaks_is_retried() {
    let script = Scripted::new(vec![
        Answer::BrokenBody(200),
        Answer::Status(200, vec![("content-type", "image/png")], "pixels"),
    ]);
    let result = scripted_account(script.clone(), no_jitter())
        .download_url("https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png")
        .await
        .unwrap();
    assert_eq!(result.body, "pixels");
    assert_eq!(script.sent_count(), 2);
}

#[tokio::test(start_paused = true)]
async fn a_deadline_during_the_hop_one_refresh_is_the_deadline() {
    use basecamp_sdk::TokenProvider;
    struct Slow;
    #[async_trait::async_trait]
    impl TokenProvider for Slow {
        async fn access_token(&self) -> Result<String, basecamp_sdk::Error> {
            Ok("stale".to_string())
        }
        fn refreshable(&self) -> bool {
            true
        }
        async fn refresh(&self) -> Result<bool, basecamp_sdk::Error> {
            tokio::time::sleep(Duration::from_secs(60)).await;
            Ok(true)
        }
    }
    let script = Scripted::new(vec![Answer::Status(401, vec![], "")]);
    let config = Config {
        operation_deadline: Some(Duration::from_secs(2)),
        ..no_jitter().with_base_url("https://3.basecampapi.com")
    };
    let error = basecamp_sdk::Client::builder(config)
        .token_provider(Slow)
        .http_client(script)
        .build()
        .unwrap()
        .for_account("999")
        .download_url("https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png")
        .await
        .unwrap_err();
    assert!(error.is_deadline_exceeded(), "{error:?}");
}
/// A refreshable provider whose one refresh answers as scripted.
struct RefreshScript {
    outcome: std::sync::Mutex<Option<Result<bool, basecamp_sdk::Error>>>,
}

#[async_trait::async_trait]
impl basecamp_sdk::TokenProvider for RefreshScript {
    async fn access_token(&self) -> Result<String, basecamp_sdk::Error> {
        Ok("t".to_string())
    }

    fn refreshable(&self) -> bool {
        true
    }

    async fn refresh(&self) -> Result<bool, basecamp_sdk::Error> {
        self.outcome
            .lock()
            .unwrap()
            .take()
            .expect("refresh asked more than once")
    }
}

fn refreshing_account(
    script: std::sync::Arc<Scripted>,
    outcome: Result<bool, basecamp_sdk::Error>,
) -> basecamp_sdk::AccountClient {
    let config = no_jitter()
        .with_base_url("https://3.basecampapi.com")
        .with_timeout(Duration::from_secs(86_400));
    basecamp_sdk::Client::builder(config)
        .token_provider(RefreshScript {
            outcome: std::sync::Mutex::new(Some(outcome)),
        })
        .http_client(script)
        .build()
        .unwrap()
        .for_account("999")
}

const BLOB: &str = "https://3.basecampapi.com/999999999/blobs/abcd1234/download/logo.png";

#[tokio::test]
async fn hop_one_replays_a_401_once_the_credentials_are_refreshed() {
    let script = Scripted::new(vec![
        Answer::Status(401, vec![], ""),
        Answer::Status(200, vec![("content-type", "image/png")], "pixels"),
    ]);
    let result = refreshing_account(script.clone(), Ok(true))
        .download_url(BLOB)
        .await
        .unwrap();
    assert_eq!(result.body, "pixels");
    assert_eq!(script.sent_count(), 2);
}

#[tokio::test]
async fn a_hop_one_refresh_that_fails_is_why_credentials_are_still_required() {
    let script = Scripted::new(vec![
        Answer::Status(401, vec![], ""),
        Answer::Status(200, vec![("content-type", "image/png")], "pixels"),
    ]);
    let error = refreshing_account(
        script.clone(),
        Err(basecamp_sdk::Error::new(
            ErrorCode::Network,
            "the token endpoint is down",
        )),
    )
    .download_url(BLOB)
    .await
    .unwrap_err();
    assert_eq!(error.code(), ErrorCode::AuthRequired);
    assert_eq!(error.http_status(), Some(401));
    assert_eq!(error.message(), "credentials could not be refreshed");
    assert!(!error.is_deadline_exceeded());
    let source = std::error::Error::source(&error).expect("the provider's failure is chained");
    assert_eq!(source.to_string(), "the token endpoint is down");
    assert_eq!(script.sent_count(), 1);
}
