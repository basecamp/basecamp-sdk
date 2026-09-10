//! SPEC §5 "Merge-Safe Write Surface (Cards)", from `conformance/tests/cards_write.json`.

mod composites_support;

use basecamp_sdk::Date;
use basecamp_sdk::models::UpdateCardRequestContent;
use basecamp_sdk::services::DateChange;
use basecamp_sdk::services::cards::UpdateCardRequest;
use composites_support::run;

const FIXTURE: &str = "cards_write";
const CARD: i64 = 1_069_479_350;

#[tokio::test]
async fn update_sends_only_what_the_caller_addressed_one_put_no_preservation_get() {
    run(
        FIXTURE,
        "update-sends-only-what-the-caller-addressed: one PUT, no preservation GET",
        |account| async move {
            account
                .cards()
                .update(
                    CARD,
                    &UpdateCardRequest {
                        title: Some("Renamed card".to_string()),
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
async fn update_verbatim_omits_due_on_raw_update_sends_one_put_with_no_get() {
    run(
        FIXTURE,
        "update-verbatim-omits-due-on: raw update sends one PUT with no GET",
        |account| async move {
            account
                .cards()
                .update_verbatim(
                    CARD,
                    &UpdateCardRequestContent {
                        title: Some("Renamed card".to_string()),
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
async fn update_explicit_clear_an_explicit_empty_due_on_goes_on_the_wire_as_empty_with_no_get() {
    run(
        FIXTURE,
        "update-explicit-clear: an explicit empty due_on goes on the wire as \"\", with no GET",
        |account| async move {
            account
                .cards()
                .update(
                    CARD,
                    &UpdateCardRequest {
                        due_on: Some(DateChange::Clear),
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
async fn update_explicit_empty_content_an_explicit_empty_content_is_sent_not_dropped() {
    run(
        FIXTURE,
        "update-explicit-empty-content: an explicit empty content is sent, not dropped",
        |account| async move {
            account
                .cards()
                .update(
                    CARD,
                    &UpdateCardRequest {
                        content: Some(String::new()),
                        due_on: Some(DateChange::Clear),
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
async fn update_explicit_empty_assignees_an_explicit_empty_assignee_list_is_sent_as_empty_list() {
    run(
        FIXTURE,
        "update-explicit-empty-assignees: an explicit empty assignee list is sent as []",
        |account| async move {
            account
                .cards()
                .update(
                    CARD,
                    &UpdateCardRequest {
                        assignee_ids: Some(Vec::new()),
                        due_on: Some(DateChange::Clear),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("updated");
}

/// Not a fixture case: a set due date goes through the generated request type as the
/// date itself, alongside a title, in the one PUT the fixture's first case pins.
#[tokio::test]
async fn update_sets_a_due_date_through_the_generated_request() {
    let server = wiremock::MockServer::start().await;
    wiremock::Mock::given(wiremock::matchers::method("PUT"))
        .and(wiremock::matchers::path(
            "/999/card_tables/cards/1069479350",
        ))
        .and(wiremock::matchers::body_json(serde_json::json!({
            "title": "Renamed card",
            "due_on": "2026-03-04"
        })))
        .respond_with(wiremock::ResponseTemplate::new(200).set_body_json({
            let mut card = fixture_body("cards_write", "update-sends-only");
            card["title"] = serde_json::json!("Renamed card");
            card["due_on"] = serde_json::json!("2026-03-04");
            card
        }))
        .expect(1)
        .mount(&server)
        .await;
    let account =
        basecamp_sdk::Client::builder(basecamp_sdk::Config::default().with_base_url(server.uri()))
            .access_token("test-token")
            .build()
            .expect("client")
            .for_account("999");
    let card = account
        .cards()
        .update(
            CARD,
            &UpdateCardRequest {
                title: Some("Renamed card".to_string()),
                due_on: Date::new(2026, 3, 4).map(DateChange::On),
                ..Default::default()
            },
        )
        .await
        .expect("updated");
    assert_eq!(card.due_on, Date::new(2026, 3, 4));
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
