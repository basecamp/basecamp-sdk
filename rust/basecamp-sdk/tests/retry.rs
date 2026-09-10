//! SPEC §7 wire-level cases, from `conformance/tests/retry.json` and `network-retry.json`.

mod support;

use std::sync::Arc;
use std::time::Duration;

use basecamp_sdk::models::CreateProjectRequestContent;
use basecamp_sdk::{Config, ErrorCode};
use support::{Answer, PROJECT, Scripted, account_with, scripted_account};
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn no_jitter() -> Config {
    let mut config = Config::default();
    config.max_jitter = Duration::ZERO;
    config
}

fn uri(request: &basecamp_sdk::http::Request<bytes::Bytes>) -> String {
    request.uri().to_string()
}

#[tokio::test(start_paused = true)]
async fn a_get_retries_on_503_with_exponential_backoff() {
    let script = Scripted::new(vec![
        Answer::Status(503, vec![], "null"),
        Answer::Status(503, vec![], "null"),
        Answer::Status(200, vec![], "[]"),
    ]);
    let started = tokio::time::Instant::now();
    let page = scripted_account(script.clone(), no_jitter())
        .projects()
        .list(&Default::default())
        .await
        .unwrap();
    assert!(page.is_empty());
    assert_eq!(script.sent_count(), 3);
    assert_eq!(
        started.elapsed(),
        Duration::from_millis(3000),
        "1s then 2s of backoff, no jitter"
    );
    let sent = script.sent.lock().unwrap();
    assert!(
        sent.iter()
            .all(|request| uri(request) == "https://3.basecampapi.com/999/projects.json")
    );
}

#[tokio::test(start_paused = true)]
async fn a_429_waits_the_retry_after_it_named() {
    let script = Scripted::new(vec![
        Answer::Status(429, vec![("retry-after", "2")], "null"),
        Answer::Status(200, vec![], PROJECT),
    ]);
    let started = tokio::time::Instant::now();
    let project = scripted_account(script.clone(), no_jitter())
        .projects()
        .get(12345)
        .await
        .unwrap();
    assert_eq!(project.id, 12345);
    assert_eq!(script.sent_count(), 2);
    assert_eq!(
        started.elapsed(),
        Duration::from_secs(2),
        "the server's delay, no jitter"
    );
}

#[tokio::test(start_paused = true)]
async fn unusable_retry_after_values_fall_through_to_backoff() {
    for value in [
        "Wed, 09 Jun 2021 10:18:14 GMT",
        "0",
        "-5",
        "120junk",
        "+5",
        "2.5",
    ] {
        let script = Scripted::new(vec![
            Answer::Status(429, vec![("retry-after", value)], "null"),
            Answer::Status(200, vec![], PROJECT),
        ]);
        let started = tokio::time::Instant::now();
        scripted_account(script.clone(), no_jitter())
            .projects()
            .get(12345)
            .await
            .unwrap();
        assert_eq!(script.sent_count(), 2, "{value}");
        assert_eq!(
            started.elapsed(),
            Duration::from_secs(1),
            "{value}: the local curve"
        );
    }
}

#[tokio::test(start_paused = true)]
async fn a_retry_after_on_503_is_honoured_too() {
    let script = Scripted::new(vec![
        Answer::Status(503, vec![("retry-after", "3")], ""),
        Answer::Status(200, vec![], PROJECT),
    ]);
    let started = tokio::time::Instant::now();
    scripted_account(script.clone(), no_jitter())
        .projects()
        .get(12345)
        .await
        .unwrap();
    assert_eq!(started.elapsed(), Duration::from_secs(3));
}

#[tokio::test]
async fn a_post_is_not_retried_unless_the_model_says_idempotent() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/999/projects.json"))
        .respond_with(ResponseTemplate::new(503).set_body_string("null"))
        .expect(1)
        .mount(&server)
        .await;
    let request = CreateProjectRequestContent {
        name: "x".into(),
        ..Default::default()
    };
    let error = account_with(&server, no_jitter())
        .projects()
        .create(&request)
        .await
        .unwrap_err();
    assert_eq!(error.http_status(), Some(503));

    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/999/projects.json"))
        .respond_with(
            ResponseTemplate::new(429)
                .set_body_string(r#"{"error": "Rate limit exceeded"}"#)
                .insert_header("Retry-After", "1"),
        )
        .expect(1)
        .mount(&server)
        .await;
    let error = account_with(&server, no_jitter())
        .projects()
        .create(&request)
        .await
        .unwrap_err();
    assert_eq!(error.http_status(), Some(429));
    assert_eq!(error.retry_after(), Some(1));
}

#[tokio::test(start_paused = true)]
async fn an_idempotent_post_is_retried() {
    let script = Scripted::new(vec![
        Answer::Status(503, vec![], ""),
        Answer::Status(204, vec![], ""),
    ]);
    scripted_account(script.clone(), no_jitter())
        .todos()
        .complete(2)
        .await
        .unwrap();
    assert_eq!(script.sent_count(), 2);
    let sent = script.sent.lock().unwrap();
    assert_eq!(
        uri(&sent[0]),
        "https://3.basecampapi.com/999/todos/2/completion.json"
    );
    assert_eq!(sent[0].method(), basecamp_sdk::http::Method::POST);
}
#[tokio::test]
async fn statuses_outside_the_declared_set_are_never_retried() {
    for (status, body, code) in [
        (404u16, r#"{"error": "Not found"}"#, ErrorCode::NotFound),
        (403, r#"{"error": "Forbidden"}"#, ErrorCode::Forbidden),
        (
            500,
            r#"{"error": "Internal Server Error"}"#,
            ErrorCode::ApiError,
        ),
        (502, r#"{"error": "Bad Gateway"}"#, ErrorCode::ApiError),
    ] {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/999/projects/12345"))
            .respond_with(ResponseTemplate::new(status).set_body_string(body))
            .expect(1)
            .mount(&server)
            .await;
        let error = account_with(&server, no_jitter())
            .projects()
            .get(12345)
            .await
            .unwrap_err();
        assert_eq!(error.code(), code);
        assert_eq!(error.http_status(), Some(status));
    }
}

#[tokio::test]
async fn max_retries_zero_sends_exactly_one_request() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects/12345"))
        .respond_with(
            ResponseTemplate::new(503).set_body_string(r#"{"error": "Service Unavailable"}"#),
        )
        .expect(1)
        .mount(&server)
        .await;
    let error = account_with(&server, no_jitter().with_max_retries(0))
        .projects()
        .get(12345)
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::ApiError);
}

#[tokio::test(start_paused = true)]
async fn the_operation_ceiling_caps_a_raised_client_budget() {
    let script = Scripted::new(vec![
        Answer::Status(503, vec![], ""),
        Answer::Status(503, vec![], ""),
    ]);
    let request = basecamp_sdk::models::UpdateAccountNameRequestContent { name: "x".into() };
    let error = scripted_account(script.clone(), no_jitter().with_max_retries(10))
        .account()
        .update_account_name(&request)
        .await
        .unwrap_err();
    assert_eq!(error.http_status(), Some(503));
    assert_eq!(script.sent_count(), 2);
    assert_eq!(
        basecamp_sdk::routes::UPDATE_ACCOUNT_NAME
            .metadata
            .retry
            .max_attempts,
        2
    );
}
#[tokio::test]
async fn update_project_client_access_declares_only_503() {
    assert_eq!(
        basecamp_sdk::routes::UPDATE_PROJECT_CLIENT_ACCESS
            .metadata
            .retry
            .retry_on,
        &[503]
    );
    let server = MockServer::start().await;
    Mock::given(method("PUT"))
        .and(path("/999/projects/1/people/client_users.json"))
        .respond_with(ResponseTemplate::new(429).set_body_string(r#"{"error": "seat limit"}"#))
        .expect(1)
        .mount(&server)
        .await;
    let error = account_with(&server, no_jitter())
        .people()
        .update_project_client_access(1, &Default::default())
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::RateLimit);
}

#[tokio::test(start_paused = true)]
async fn network_errors_retry_under_the_idempotency_gate() {
    let script = Scripted::new(vec![
        Answer::NetworkError,
        Answer::NetworkError,
        Answer::Status(200, vec![], PROJECT),
    ]);
    let project = scripted_account(script.clone(), no_jitter())
        .projects()
        .get(12345)
        .await
        .unwrap();
    assert_eq!(project.id, 12345);
    assert_eq!(script.sent_count(), 3);

    let script = Scripted::new(vec![Answer::NetworkError]);
    let request = CreateProjectRequestContent {
        name: "x".into(),
        ..Default::default()
    };
    let error = scripted_account(script.clone(), no_jitter())
        .projects()
        .create(&request)
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Network);
    assert!(error.is_retryable());
    assert_eq!(script.sent_count(), 1);
}

#[tokio::test(start_paused = true)]
async fn the_operation_deadline_bounds_the_whole_call() {
    let script = Scripted::new(vec![
        Answer::Status(429, vec![("retry-after", "30")], ""),
        Answer::Status(200, vec![], PROJECT),
    ]);
    let mut config = no_jitter();
    config.operation_deadline = Some(Duration::from_secs(5));
    let error = scripted_account(script.clone(), config)
        .projects()
        .get(12345)
        .await
        .unwrap_err();
    assert!(error.is_deadline_exceeded());
    assert_eq!(error.code(), ErrorCode::Network);
    assert!(!error.is_retryable());
    assert_eq!(
        script.sent_count(),
        1,
        "the wait outlives the deadline, so nothing more is sent"
    );
}
#[tokio::test(start_paused = true)]
async fn hooks_see_every_attempt_and_the_retry_delay() {
    use basecamp_sdk::hooks::{Hooks, OperationInfo, OperationResult, RequestInfo, RequestResult};
    use std::sync::Mutex;

    #[derive(Default)]
    struct Log(Mutex<Vec<String>>);

    impl Hooks for Log {
        fn on_operation_start(&self, info: &OperationInfo) {
            self.0
                .lock()
                .unwrap()
                .push(format!("op start {} {}", info.service, info.operation));
        }
        fn on_operation_end(&self, info: &OperationInfo, result: &OperationResult<'_>) {
            self.0.lock().unwrap().push(format!(
                "op end {} ok={}",
                info.operation,
                result.error.is_none()
            ));
        }
        fn on_request_start(&self, info: &RequestInfo) {
            self.0
                .lock()
                .unwrap()
                .push(format!("req start {}", info.attempt));
        }
        fn on_request_end(&self, info: &RequestInfo, result: &RequestResult<'_>) {
            self.0.lock().unwrap().push(format!(
                "req end {} {:?}",
                info.attempt,
                result.status.map(|s| s.as_u16())
            ));
        }
        fn on_retry(
            &self,
            info: &RequestInfo,
            next: u32,
            _error: &basecamp_sdk::Error,
            delay: Duration,
        ) {
            self.0.lock().unwrap().push(format!(
                "retry after {} -> {} in {delay:?}",
                info.attempt, next
            ));
        }
    }

    let script = Scripted::new(vec![
        Answer::Status(503, vec![], ""),
        Answer::Status(200, vec![], PROJECT),
    ]);
    let log = Arc::new(Log::default());
    let client =
        basecamp_sdk::Client::builder(no_jitter().with_base_url("https://3.basecampapi.com"))
            .access_token("t")
            .http_client(script)
            .hooks(log.clone())
            .build()
            .unwrap()
            .for_account("999");
    client.projects().get(12345).await.unwrap();
    assert_eq!(
        *log.0.lock().unwrap(),
        [
            "op start Projects GetProject",
            "req start 1",
            "req end 1 Some(503)",
            "retry after 1 -> 2 in 1s",
            "req start 2",
            "req end 2 Some(200)",
            "op end GetProject ok=true",
        ]
    );
}
