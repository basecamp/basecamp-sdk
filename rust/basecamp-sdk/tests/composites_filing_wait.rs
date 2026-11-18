//! The filing waits (SPEC §18, Appendix F "Filing Waits"): a move or copy started once and
//! polled until it settles.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use std::time::Duration;

use basecamp_sdk::models::{CopyRecordingRequestContent, MoveRecordingRequestContent};
use basecamp_sdk::services::filing_wait::{FilingContainer, FilingError, FilingWaitOptions};
use basecamp_sdk::{Error, ErrorCode};
use support::account;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn filing(status: &str) -> serde_json::Value {
    serde_json::json!({"id": 4, "status": status, "type": "Copy", "source_recording_id": 456, "url": "u"})
}

fn copy_request() -> CopyRecordingRequestContent {
    CopyRecordingRequestContent {
        filing_destination_id: Some(789),
        ..Default::default()
    }
}

async fn copy_and_wait(
    server: &MockServer,
    options: Option<&FilingWaitOptions>,
) -> Result<(), Error> {
    account(server)
        .recordings()
        .copy_and_wait(456, &copy_request(), options)
        .await
        .map(|_| ())
}

async fn mount_start(server: &MockServer, verb_path: &str, status: u16, body: serde_json::Value) {
    Mock::given(method("POST"))
        .and(path(verb_path))
        .respond_with(ResponseTemplate::new(status).set_body_json(body))
        .expect(1)
        .mount(server)
        .await;
}

#[tokio::test]
async fn polls_until_the_copy_completes() {
    let server = MockServer::start().await;
    mount_start(
        &server,
        "/999/recordings/456/copies.json",
        201,
        filing("pending"),
    )
    .await;
    let mut completed = filing("completed");
    completed["destination_recording"] =
        serde_json::json!({"id": 900, "type": "Message", "url": "u", "app_url": "a"});
    Mock::given(method("GET"))
        .and(path("/999/filings/4"))
        .respond_with(ResponseTemplate::new(200).set_body_json(filing("processing")))
        .up_to_n_times(1)
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/999/filings/4"))
        .respond_with(ResponseTemplate::new(200).set_body_json(completed))
        .mount(&server)
        .await;

    let result = account(&server)
        .recordings()
        .copy_and_wait(456, &copy_request(), None)
        .await
        .unwrap();
    assert_eq!(result.destination_recording.unwrap().id, 900);
}

#[tokio::test]
async fn a_failed_move_is_failed() {
    let server = MockServer::start().await;
    mount_start(
        &server,
        "/999/recordings/456/moves.json",
        201,
        filing("pending"),
    )
    .await;
    Mock::given(method("GET"))
        .and(path("/999/filings/4"))
        .respond_with(ResponseTemplate::new(200).set_body_json(filing("failed")))
        .mount(&server)
        .await;

    let request = MoveRecordingRequestContent {
        filing_destination_id: Some(789),
        ..Default::default()
    };
    let error = account(&server)
        .recordings()
        .move_and_wait(456, &request, None)
        .await
        .unwrap_err();
    assert!(matches!(FilingError::of(&error), Some(FilingError::Failed(f)) if f.id == 4));
}

#[tokio::test]
async fn the_wait_ends_while_processing() {
    let server = MockServer::start().await;
    mount_start(
        &server,
        "/999/recordings/456/copies.json",
        201,
        filing("pending"),
    )
    .await;
    Mock::given(method("GET"))
        .and(path("/999/filings/4"))
        .respond_with(ResponseTemplate::new(200).set_body_json(filing("processing")))
        .mount(&server)
        .await;

    let options = FilingWaitOptions::with_timeout(Duration::from_millis(600));
    let error = copy_and_wait(&server, Some(&options)).await.unwrap_err();
    assert!(matches!(
        FilingError::of(&error),
        Some(FilingError::Unsettled { filing, cause: None }) if filing.status == "processing"
    ));
    assert!(!error.is_retryable());
}

#[tokio::test]
async fn a_failed_poll_is_unsettled_not_refused() {
    let server = MockServer::start().await;
    mount_start(
        &server,
        "/999/recordings/456/copies.json",
        201,
        filing("pending"),
    )
    .await;
    Mock::given(method("GET"))
        .and(path("/999/filings/4"))
        .respond_with(
            ResponseTemplate::new(404).set_body_json(serde_json::json!({"error": "Not Found"})),
        )
        .mount(&server)
        .await;

    let error = copy_and_wait(&server, None).await.unwrap_err();
    assert!(matches!(
        FilingError::of(&error),
        Some(FilingError::Unsettled { cause: Some(cause), .. }) if cause.code() == ErrorCode::NotFound
    ));
    assert_eq!(error.code(), ErrorCode::ApiError);
}

#[tokio::test]
async fn a_refused_start_polls_nothing() {
    for status in [403, 404, 422] {
        let server = MockServer::start().await;
        mount_start(
            &server,
            "/999/recordings/456/copies.json",
            status,
            serde_json::json!({}),
        )
        .await;

        let error = copy_and_wait(&server, None).await.unwrap_err();
        assert!(
            matches!(FilingError::of(&error), Some(FilingError::Refused(cause)) if cause.http_status() == Some(status))
        );
    }
}

#[tokio::test]
async fn a_start_that_may_have_filed_is_sent_once() {
    for status in [500, 503] {
        let server = MockServer::start().await;
        mount_start(
            &server,
            "/999/recordings/456/copies.json",
            status,
            serde_json::json!({}),
        )
        .await;

        let error = copy_and_wait(&server, None).await.unwrap_err();
        assert!(matches!(
            FilingError::of(&error),
            Some(FilingError::MayHaveStarted(_))
        ));
        assert!(!error.is_retryable());
    }
}

#[test]
fn filing_containers() {
    let rows = [
        ("Kanban::Card", "Kanban::Triage", Some("kanban_board")),
        ("Kanban::Column", "Kanban::Board", Some("kanban_board")),
        ("Message", "Message::Board", Some("message_board")),
        ("Document", "Vault", Some("vault")),
        ("Upload", "Vault", Some("vault")),
        ("CloudFile", "Vault", Some("vault")),
        ("GoogleDocument", "Vault", Some("vault")),
        ("Vault", "Vault", Some("vault")),
        ("Schedule::Entry", "Schedule", Some("schedule")),
        ("Question", "Questionnaire", Some("questionnaire")),
        ("Inbox::Forward", "Inbox", Some("inbox")),
        ("Todolist", "Todoset", Some("todoset")),
        ("Todolist", "Todolist", Some("todolist")),
        ("Todo", "Todolist", Some("todolist")),
        ("Vault", "", None),
        ("Message::Board", "", None),
        ("Kanban::Triage", "Kanban::Board", None),
        ("Comment", "Message", None),
        ("Question::Answer", "Question", None),
    ];
    for (recording, parent, want) in rows {
        let got = FilingContainer::of(recording, parent);
        assert_eq!(
            got.map(FilingContainer::dock_name),
            want,
            "{recording} under {parent}"
        );
        if let Some(container) = got {
            assert_eq!(container.takes_folders(), want == Some("vault"));
        }
    }
}
