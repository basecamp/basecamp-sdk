//! Updating a boost: the body carries `content` alone, the 200 decodes to the boost, and 403,
//! 404 and a bodiless 422 map to their codes.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use basecamp_sdk::ErrorCode;
use basecamp_sdk::generated::types::UpdateBoostRequestContent;
use serde_json::{Value, json};
use support::account;
use wiremock::matchers::{body_json, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

const GET: &str = include_str!("../../../spec/fixtures/boosts/get.json");

fn request(content: &str) -> UpdateBoostRequestContent {
    UpdateBoostRequestContent {
        content: content.to_string(),
    }
}

#[tokio::test]
async fn update_sends_content_alone_and_decodes_the_boost() {
    let server = MockServer::start().await;
    let mut fixture: Value = serde_json::from_str(GET).unwrap();
    fixture["content"] = json!("Done!");
    Mock::given(method("PUT"))
        .and(path("/999/boosts/500"))
        .and(body_json(json!({"content": "Done!"})))
        .respond_with(ResponseTemplate::new(200).set_body_json(fixture))
        .expect(1)
        .mount(&server)
        .await;
    let boost = account(&server)
        .boosts()
        .update(500, &request("Done!"))
        .await
        .unwrap();
    assert_eq!(boost.id, 1_069_479_500);
    assert_eq!(boost.content.as_deref(), Some("Done!"));
}

#[tokio::test]
async fn update_maps_403_404_and_a_bodiless_422() {
    for (status, code) in [
        (403, ErrorCode::Forbidden),
        (404, ErrorCode::NotFound),
        (422, ErrorCode::Validation),
    ] {
        let server = MockServer::start().await;
        Mock::given(method("PUT"))
            .and(path("/999/boosts/500"))
            .respond_with(ResponseTemplate::new(status))
            .expect(1)
            .mount(&server)
            .await;
        let error = account(&server)
            .boosts()
            .update(500, &request("Done!"))
            .await
            .unwrap_err();
        assert_eq!(error.code(), code);
        assert_eq!(error.http_status(), Some(status));
    }
}
