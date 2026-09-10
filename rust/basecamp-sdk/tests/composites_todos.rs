//! SPEC §5 "Merge-Safe Write Surface (Todos)", from `conformance/tests/todos_write.json`.

mod composites_support;

use basecamp_sdk::ErrorCode;
use basecamp_sdk::models::ReplaceTodoRequestContent;
use basecamp_sdk::services::todos::UpdateTodoRequest;
use composites_support::run;

const FIXTURE: &str = "todos_write";

#[tokio::test]
async fn update_merge_content_only_update_preserves_every_unset_field() {
    run(
        FIXTURE,
        "update-merge: content-only update preserves every unset field",
        |account| async move {
            account
                .todos()
                .update(
                    456,
                    &UpdateTodoRequest {
                        content: Some("New title".to_string()),
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
async fn edit_clear_setting_fields_empty_clears_them_while_everything_else_is_preserved() {
    run(
        FIXTURE,
        "edit-clear: setting fields empty clears them while everything else is preserved",
        |account| async move {
            account
                .todos()
                .edit(456, |todo| {
                    todo.description = String::new();
                    todo.assignee_ids = Vec::new();
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
                .todos()
                .replace(
                    456,
                    &ReplaceTodoRequestContent {
                        content: "The whole new todo".to_string(),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("replaced");
}

async fn update_is_refused(name: &str) {
    let error = run(FIXTURE, name, |account| async move {
        account
            .todos()
            .update(
                456,
                &UpdateTodoRequest {
                    content: Some("New title".to_string()),
                    ..Default::default()
                },
            )
            .await
    })
    .await
    .expect_err("refused");
    assert_eq!(error.code(), ErrorCode::ApiError);
    assert_eq!(error.http_status(), None);
    assert!(!error.is_retryable());
}

#[tokio::test]
async fn update_kill_an_array_description_is_refused_before_the_full_replace_put() {
    update_is_refused("update-kill: an array description is refused before the full-replace PUT")
        .await;
}

#[tokio::test]
async fn update_kill_an_empty_object_description_is_refused_not_coalesced_to_empty() {
    update_is_refused(
        "update-kill: an empty-object description is refused, not coalesced to empty",
    )
    .await;
}

#[tokio::test]
async fn update_kill_a_bare_scalar_description_is_refused_before_the_full_replace_put() {
    update_is_refused(
        "update-kill: a bare-scalar description is refused before the full-replace PUT",
    )
    .await;
}

#[tokio::test]
async fn edit_aborts_before_the_put_when_the_closure_fails() {
    let server = wiremock::MockServer::start().await;
    let fixture = std::fs::read_to_string(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../conformance/tests/todos_write.json"
    ))
    .expect("fixture");
    let cases: Vec<serde_json::Value> = serde_json::from_str(&fixture).expect("json");
    wiremock::Mock::given(wiremock::matchers::method("GET"))
        .respond_with(
            wiremock::ResponseTemplate::new(200)
                .set_body_json(&cases[0]["mockResponses"][0]["body"]),
        )
        .mount(&server)
        .await;
    let account =
        basecamp_sdk::Client::builder(basecamp_sdk::Config::default().with_base_url(server.uri()))
            .access_token("test-token")
            .build()
            .expect("client")
            .for_account("999");
    let error = account
        .todos()
        .edit(456, |_| Err(basecamp_sdk::Error::usage("changed my mind")))
        .await
        .expect_err("aborted");
    assert_eq!(error.code(), ErrorCode::Usage);
    let requests = server.received_requests().await.unwrap_or_default();
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0].method.as_str(), "GET");
}
