//! SPEC §5 "Merge-Safe Write Surface (Documents)", from
//! `conformance/tests/documents_write.json`.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod composites_support;

use basecamp_sdk::models::ReplaceDocumentRequestContent;
use basecamp_sdk::services::documents::UpdateDocumentRequest;
use basecamp_sdk::{Client, Config, ErrorCode};
use composites_support::run;
use wiremock::matchers::method;
use wiremock::{Mock, MockServer, ResponseTemplate};

const FIXTURE: &str = "documents_write";

#[tokio::test]
async fn update_merge_title_only_update_preserves_the_content() {
    run(
        FIXTURE,
        "update-merge: title-only update preserves the content",
        |account| async move {
            account
                .documents()
                .update(
                    456,
                    &UpdateDocumentRequest {
                        title: Some("Q3 Plan".to_string()),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("updated");
}

#[tokio::test]
async fn edit_clear_setting_content_empty_clears_it_while_the_title_is_preserved() {
    run(
        FIXTURE,
        "edit-clear: setting content empty clears it while the title is preserved",
        |account| async move {
            account
                .documents()
                .edit(456, |document| {
                    document.content = String::new();
                    Ok(())
                })
                .await
        },
    )
    .await
    .expect("edited");
}

#[tokio::test]
async fn replace_omission_clears_sparse_replace_sends_the_request_verbatim_with_no_get() {
    run(
        FIXTURE,
        "replace-omission-clears: sparse replace sends the request verbatim with no GET",
        |account| async move {
            account
                .documents()
                .replace(
                    456,
                    &ReplaceDocumentRequestContent {
                        title: Some("The whole new document".to_string()),
                        content: None,
                    },
                )
                .await
        },
    )
    .await
    .expect("replaced");
}

/// Not a fixture case: SPEC §5 says an absent `title` in a 2xx read is a malformed
/// response, never coalesced to `""`, so the composite must stop before the PUT.
#[tokio::test]
async fn update_refuses_a_read_back_without_a_title() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(200).set_body_json({
            let mut document = fixture_body("documents_write", "update-merge");
            document.as_object_mut().expect("object").remove("title");
            document
        }))
        .mount(&server)
        .await;
    let account = Client::builder(Config::default().with_base_url(server.uri()))
        .access_token("test-token")
        .build()
        .expect("client")
        .for_account("999");
    let error = account
        .documents()
        .update(
            456,
            &UpdateDocumentRequest {
                content: Some("<div>New</div>".to_string()),
                ..Default::default()
            },
        )
        .await
        .expect_err("refused");
    assert_eq!(error.code(), ErrorCode::ApiError);
    assert_eq!(error.http_status(), None);
    assert!(error.message().contains("title"));
    let requests = server.received_requests().await.unwrap_or_default();
    assert_eq!(requests.len(), 1);
}

fn fixture_body(file: &str, case: &str) -> serde_json::Value {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../conformance/tests")
        .join(format!("{file}.json"));
    let fixture: serde_json::Value =
        serde_json::from_str(&std::fs::read_to_string(path).expect("fixture file"))
            .expect("fixture JSON");
    let tests = fixture.get("tests").cloned().unwrap_or(fixture);
    let case = tests
        .as_array()
        .expect("cases")
        .iter()
        .find(|c| {
            c["name"]
                .as_str()
                .is_some_and(|name| name.starts_with(case))
        })
        .expect("case");
    case["mockResponses"][0]["body"].clone()
}
