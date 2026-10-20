//! Embeds: create and get, with the sgid sent as one path segment.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use basecamp_sdk::ErrorCode;
use basecamp_sdk::generated::types::CreateEmbedRequestContent;
use serde_json::{Value, json};
use support::account;
use wiremock::matchers::{body_json, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

const PENDING: &str = include_str!("../../../spec/fixtures/embeds/pending.json");
const ACTIVE: &str = include_str!("../../../spec/fixtures/embeds/active.json");
const YOUTUBE_URL: &str = "https://www.youtube.com/watch?v=dQw4w9WgXcQ";

fn fixture(raw: &str) -> Value {
    serde_json::from_str(raw).unwrap()
}

fn create_request(url: &str) -> CreateEmbedRequestContent {
    CreateEmbedRequestContent {
        url: url.to_string(),
    }
}

#[tokio::test]
async fn create_sends_the_url_and_decodes_the_pending_embed() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/999/embeds.json"))
        .and(body_json(json!({"url": YOUTUBE_URL})))
        .respond_with(ResponseTemplate::new(200).set_body_json(fixture(PENDING)))
        .expect(1)
        .mount(&server)
        .await;
    let embed = account(&server)
        .embeds()
        .create(&create_request(YOUTUBE_URL))
        .await
        .unwrap();
    assert_eq!(embed.status, "pending");
    assert_eq!(embed.sgid, fixture(PENDING)["sgid"]);
    assert!(embed.content_type.is_none() && embed.r#type.is_none() && embed.image_url.is_none());
}

#[tokio::test]
async fn create_returns_an_existing_active_embed() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/999/embeds.json"))
        .respond_with(ResponseTemplate::new(200).set_body_json(fixture(ACTIVE)))
        .expect(1)
        .mount(&server)
        .await;
    let embed = account(&server)
        .embeds()
        .create(&create_request(YOUTUBE_URL))
        .await
        .unwrap();
    assert_eq!(embed.status, "active");
    assert_eq!(embed.content_type.as_deref(), Some("embed/youtube-video"));
    assert!(embed.image_url.is_none());
}

#[tokio::test]
async fn create_maps_422_and_429() {
    for (status, message, code) in [
        (
            422,
            "Basecamp can't embed this URL. Link to it instead.",
            ErrorCode::Validation,
        ),
        (
            429,
            "Too many new embeds. Try again later.",
            ErrorCode::RateLimit,
        ),
    ] {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .and(path("/999/embeds.json"))
            .respond_with(ResponseTemplate::new(status).set_body_json(json!({"error": message})))
            .expect(1)
            .mount(&server)
            .await;
        let error = account(&server)
            .embeds()
            .create(&create_request("https://example.com/page"))
            .await
            .unwrap_err();
        assert_eq!(error.code(), code);
        assert_eq!(error.http_status(), Some(status));
        assert!(error.message().contains(message), "{}", error.message());
    }
}

#[tokio::test]
async fn get_escapes_the_sgid_as_one_path_segment() {
    let sgid = fixture(PENDING)["sgid"].as_str().unwrap().to_string();
    let escaped = sgid.replace('/', "%2F").replace('=', "%3D");
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path(format!("/999/embeds/{escaped}")))
        .respond_with(ResponseTemplate::new(200).set_body_json(fixture(ACTIVE)))
        .expect(1)
        .mount(&server)
        .await;
    let embed = account(&server).embeds().get(&sgid).await.unwrap();
    assert_eq!(embed.status, "active");
}

#[tokio::test]
async fn get_maps_404() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/embeds/missing"))
        .respond_with(ResponseTemplate::new(404).set_body_json(json!({"error": "Not found"})))
        .expect(1)
        .mount(&server)
        .await;
    let error = account(&server).embeds().get("missing").await.unwrap_err();
    assert_eq!(error.code(), ErrorCode::NotFound);
}
