//! SPEC §4: the 401 refresh-and-replay, its budget gate, and coalescing.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

use async_trait::async_trait;
use basecamp_sdk::{Client, Config, Error, ErrorCode, TokenProvider};
use support::PROJECT;
use wiremock::matchers::{header, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

struct Rotating {
    tokens: std::sync::Mutex<Vec<&'static str>>,
    refreshes: AtomicUsize,
    refresh_delay: Duration,
}

#[async_trait]
impl TokenProvider for Rotating {
    async fn access_token(&self) -> Result<String, Error> {
        Ok(self.tokens.lock().unwrap()[0].to_string())
    }

    fn refreshable(&self) -> bool {
        true
    }

    async fn refresh(&self) -> Result<bool, Error> {
        tokio::time::sleep(self.refresh_delay).await;
        self.refreshes.fetch_add(1, Ordering::SeqCst);
        let mut tokens = self.tokens.lock().unwrap();
        if tokens.len() > 1 {
            tokens.remove(0);
            Ok(true)
        } else {
            Ok(false)
        }
    }
}

fn rotating(tokens: &[&'static str], refresh_delay: Duration) -> Arc<Rotating> {
    Arc::new(Rotating {
        tokens: std::sync::Mutex::new(tokens.to_vec()),
        refreshes: AtomicUsize::new(0),
        refresh_delay,
    })
}

async fn mount_401_then_200(server: &MockServer) {
    Mock::given(method("GET"))
        .and(path("/999/projects/12345"))
        .and(header("Authorization", "Bearer stale"))
        .respond_with(ResponseTemplate::new(401).set_body_string(r#"{"error": "Unauthorized"}"#))
        .mount(server)
        .await;
    Mock::given(method("GET"))
        .and(path("/999/projects/12345"))
        .and(header("Authorization", "Bearer fresh"))
        .respond_with(ResponseTemplate::new(200).set_body_string(PROJECT))
        .mount(server)
        .await;
}

fn client(
    server: &MockServer,
    provider: Arc<Rotating>,
    max_retries: u32,
) -> basecamp_sdk::AccountClient {
    Client::builder(
        Config::default()
            .with_base_url(server.uri())
            .with_max_retries(max_retries)
            .with_timeout(Duration::from_secs(86_400)),
    )
    .token_provider(provider)
    .build()
    .unwrap()
    .for_account("999")
}

#[tokio::test]
async fn a_401_is_replayed_once_with_the_refreshed_token() {
    let server = MockServer::start().await;
    mount_401_then_200(&server).await;
    let provider = rotating(&["stale", "fresh"], Duration::ZERO);
    let project = client(&server, provider.clone(), 3)
        .projects()
        .get(12345)
        .await
        .unwrap();
    assert_eq!(project.id, 12345);
    assert_eq!(provider.refreshes.load(Ordering::SeqCst), 1);
    assert_eq!(server.received_requests().await.unwrap().len(), 2);
}

#[tokio::test]
async fn a_failed_refresh_surfaces_auth_required_without_a_replay() {
    let server = MockServer::start().await;
    mount_401_then_200(&server).await;
    let provider = rotating(&["stale"], Duration::ZERO);
    let error = client(&server, provider.clone(), 3)
        .projects()
        .get(12345)
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::AuthRequired);
    assert_eq!(error.http_status(), Some(401));
    assert_eq!(server.received_requests().await.unwrap().len(), 1);
}

#[tokio::test]
async fn the_budget_gate_is_checked_before_refreshing() {
    let server = MockServer::start().await;
    mount_401_then_200(&server).await;
    let provider = rotating(&["stale", "fresh"], Duration::ZERO);
    let error = client(&server, provider.clone(), 1)
        .projects()
        .get(12345)
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::AuthRequired);
    assert_eq!(
        provider.refreshes.load(Ordering::SeqCst),
        0,
        "no refresh is spent when nothing can use it"
    );
    assert_eq!(server.received_requests().await.unwrap().len(), 1);
}

#[tokio::test(start_paused = true)]
async fn concurrent_401s_coalesce_into_one_refresh() {
    use support::{Answer, Scripted};
    let script = Scripted::new(vec![
        Answer::Status(401, vec![], r#"{"error": "Unauthorized"}"#),
        Answer::Status(401, vec![], r#"{"error": "Unauthorized"}"#),
        Answer::Status(401, vec![], r#"{"error": "Unauthorized"}"#),
        Answer::Status(200, vec![], PROJECT),
        Answer::Status(200, vec![], PROJECT),
        Answer::Status(200, vec![], PROJECT),
    ]);
    let provider = rotating(&["stale", "fresh"], Duration::from_millis(50));
    let account = Client::builder(Config::default())
        .token_provider(provider.clone())
        .http_client(script.clone())
        .build()
        .unwrap()
        .for_account("999");
    let (projects_a, projects_b, projects_c) =
        (account.projects(), account.projects(), account.projects());
    let (a, b, c) = tokio::join!(
        projects_a.get(12345),
        projects_b.get(12345),
        projects_c.get(12345)
    );
    assert!(a.is_ok() && b.is_ok() && c.is_ok(), "{a:?} {b:?} {c:?}");
    assert_eq!(
        provider.refreshes.load(Ordering::SeqCst),
        1,
        "one refresh serves every waiter"
    );
    assert_eq!(script.sent_count(), 6);
    let sent = script.sent.lock().unwrap();
    let tokens: Vec<&str> = sent
        .iter()
        .map(|request| request.headers()["authorization"].to_str().unwrap())
        .collect();
    assert_eq!(
        tokens,
        [
            "Bearer stale",
            "Bearer stale",
            "Bearer stale",
            "Bearer fresh",
            "Bearer fresh",
            "Bearer fresh"
        ]
    );
}
#[tokio::test]
async fn a_static_token_is_never_refreshed() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects/12345"))
        .respond_with(ResponseTemplate::new(401).set_body_string(r#"{"error": "Unauthorized"}"#))
        .expect(1)
        .mount(&server)
        .await;
    let error = support::account(&server)
        .projects()
        .get(12345)
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::AuthRequired);
}

#[tokio::test(start_paused = true)]
async fn a_failed_refresh_is_shared_with_every_waiter() {
    use support::{Answer, Scripted};
    let script = Scripted::new(vec![
        Answer::Status(401, vec![], ""),
        Answer::Status(401, vec![], ""),
        Answer::Status(401, vec![], ""),
    ]);
    let provider = rotating(&["stale"], Duration::from_millis(50));
    let account = Client::builder(Config::default())
        .token_provider(provider.clone())
        .http_client(script.clone())
        .build()
        .unwrap()
        .for_account("999");
    let (projects_a, projects_b, projects_c) =
        (account.projects(), account.projects(), account.projects());
    let (a, b, c) = tokio::join!(projects_a.get(1), projects_b.get(1), projects_c.get(1));
    for outcome in [a, b, c] {
        assert_eq!(outcome.unwrap_err().code(), ErrorCode::AuthRequired);
    }
    assert_eq!(
        provider.refreshes.load(Ordering::SeqCst),
        1,
        "one failed refresh answers the wave"
    );
    assert_eq!(script.sent_count(), 3);
}

#[tokio::test(start_paused = true)]
async fn a_request_authenticated_after_a_wave_is_not_bound_by_its_verdict() {
    use support::{Answer, Scripted};
    let script = Scripted::new(vec![
        Answer::Status(401, vec![], ""),
        Answer::Status(401, vec![], ""),
        Answer::Status(401, vec![], ""),
        Answer::Status(200, vec![], PROJECT),
    ]);
    let provider = rotating(&["stale", "stale", "fresh"], Duration::ZERO);
    let account = Client::builder(Config::default())
        .token_provider(provider.clone())
        .http_client(script.clone())
        .build()
        .unwrap()
        .for_account("999");
    // The first refresh rotates to another stale token and reports success; the replay
    // meets a second 401 and has spent its one refresh.
    assert_eq!(
        account.projects().get(1).await.unwrap_err().code(),
        ErrorCode::AuthRequired
    );
    assert_eq!(provider.refreshes.load(Ordering::SeqCst), 1);
    // A request authenticated after that attempt gets its own refresh.
    let project = account.projects().get(1).await.unwrap();
    assert_eq!(project.id, 12345);
    assert_eq!(provider.refreshes.load(Ordering::SeqCst), 2);
    assert_eq!(script.sent_count(), 4);
}

#[tokio::test(start_paused = true)]
async fn a_request_after_a_failed_refresh_starts_another_attempt() {
    use support::{Answer, Scripted};
    let script = Scripted::new(vec![
        Answer::Status(401, vec![], ""),
        Answer::Status(401, vec![], ""),
        Answer::Status(200, vec![], PROJECT),
    ]);
    let provider = rotating(&["stale"], Duration::ZERO);
    let account = Client::builder(Config::default())
        .token_provider(provider.clone())
        .http_client(script.clone())
        .build()
        .unwrap()
        .for_account("999");
    assert_eq!(
        account.projects().get(1).await.unwrap_err().code(),
        ErrorCode::AuthRequired
    );
    assert_eq!(provider.refreshes.load(Ordering::SeqCst), 1);
    provider.tokens.lock().unwrap().push("fresh");
    let project = account.projects().get(1).await.unwrap();
    assert_eq!(project.id, 12345);
    assert_eq!(
        provider.refreshes.load(Ordering::SeqCst),
        2,
        "the issuer recovered, so the next 401 refreshes again"
    );
}

#[tokio::test]
async fn a_401_on_a_non_idempotent_post_is_replayed_after_a_refresh() {
    use basecamp_sdk::models::CreateProjectRequestContent;
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/999/projects.json"))
        .and(header("Authorization", "Bearer stale"))
        .respond_with(ResponseTemplate::new(401).set_body_string(r#"{"error": "Unauthorized"}"#))
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/999/projects.json"))
        .and(header("Authorization", "Bearer fresh"))
        .respond_with(ResponseTemplate::new(201).set_body_string(PROJECT))
        .mount(&server)
        .await;
    let provider = rotating(&["stale", "fresh"], Duration::ZERO);
    let request = CreateProjectRequestContent {
        name: "x".into(),
        ..Default::default()
    };
    let project = client(&server, provider.clone(), 3)
        .projects()
        .create(&request)
        .await
        .unwrap();
    assert_eq!(project.id, 12345);
    assert_eq!(provider.refreshes.load(Ordering::SeqCst), 1);
    assert_eq!(
        server.received_requests().await.unwrap().len(),
        2,
        "the replay spends the caller's budget, not the transient-retry ceiling"
    );

    let provider = rotating(&["stale", "fresh"], Duration::ZERO);
    let error = client(&server, provider.clone(), 1)
        .projects()
        .create(&request)
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::AuthRequired);
    assert_eq!(provider.refreshes.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn a_refresh_outlives_the_request_that_started_it() {
    let server = MockServer::start().await;
    mount_401_then_200(&server).await;
    let provider = rotating(&["stale", "fresh"], Duration::from_millis(300));
    let account = Client::builder(
        Config::default()
            .with_base_url(server.uri())
            .with_timeout(Duration::from_secs(86_400)),
    )
    .operation_deadline(Duration::from_millis(50))
    .token_provider(provider.clone())
    .build()
    .unwrap()
    .for_account("999");
    let mut outcomes = Vec::new();
    for _ in 0..12 {
        let outcome = account.projects().get(12345).await;
        let done = outcome.is_ok();
        outcomes.push(outcome.map(|_| ()).map_err(|e| e.is_deadline_exceeded()));
        if done {
            break;
        }
    }
    assert_eq!(
        provider.refreshes.load(Ordering::SeqCst),
        1,
        "the refresh a deadline cut short is finished by later requests, not restarted: {outcomes:?}"
    );
    assert!(outcomes.last().unwrap().is_ok(), "{outcomes:?}");
    assert!(
        outcomes.len() > 1 && outcomes[0] == Err(true),
        "the first request must have been cut short while the refresh ran: {outcomes:?}"
    );
    assert!(
        outcomes[..outcomes.len() - 1]
            .iter()
            .all(|outcome| *outcome == Err(true)),
        "{outcomes:?}"
    );
}
