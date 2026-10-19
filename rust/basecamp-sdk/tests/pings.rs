//! Pings (their bucket type and wire path are "Circle"): the fixtures the Go reference is
//! tested against decode, a create carries `user_ids` and `new_group` only as set, and 403,
//! 404 and 422 map to their codes.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use basecamp_sdk::ErrorCode;
use basecamp_sdk::generated::types::CreatePingRequestContent;
use serde_json::{Value, json};
use support::account;
use wiremock::matchers::{body_json, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

const LIST: &str = include_str!("../../../spec/fixtures/pings/list.json");
const GET: &str = include_str!("../../../spec/fixtures/pings/get.json");

fn fixture(raw: &str) -> Value {
    serde_json::from_str(raw).unwrap()
}

#[tokio::test]
async fn list_decodes_a_named_group_and_a_direct_ping() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/circles.json"))
        .respond_with(ResponseTemplate::new(200).set_body_json(fixture(LIST)))
        .expect(1)
        .mount(&server)
        .await;
    let pings = account(&server).pings().list().await.unwrap();
    assert_eq!(pings.len(), 2);
    let (group, direct) = (&pings[0], &pings[1]);
    assert_eq!(group.id, 2_085_958_600);
    assert_eq!(group.title, "Launch crew");
    assert!(group.named);
    assert_eq!(group.people.len(), 2);
    assert_eq!(group.people[1].name.expose(), "Cheryl Walters");
    assert!(group.edit_title_url.is_some() && group.image_url.is_some());
    assert!(!direct.named);
    assert!(direct.edit_title_url.is_none() && direct.image_url.is_none());
    assert_eq!(
        direct.links.lines,
        "https://3.basecampapi.com/195539477/buckets/2085958601/chats/1069479601/lines.json"
    );
}

#[tokio::test]
async fn get_decodes_the_ping_and_names_its_campfire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/circles/2085958600"))
        .respond_with(ResponseTemplate::new(200).set_body_json(fixture(GET)))
        .expect(1)
        .mount(&server)
        .await;
    let ping = account(&server).pings().get(2_085_958_600).await.unwrap();
    assert_eq!(ping.id, 2_085_958_600);
    assert_eq!(ping.title, "Launch crew");
    assert_eq!(
        ping.room_url,
        "https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600.json"
    );
    assert_eq!(ping.people.len(), 2);
}

#[tokio::test]
async fn get_maps_404_and_403() {
    for (status, body, code) in [
        (
            404,
            json!({"error": "Record not found"}),
            ErrorCode::NotFound,
        ),
        (
            403,
            json!({"error": "Pings have been disabled"}),
            ErrorCode::Forbidden,
        ),
    ] {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/999/circles/7"))
            .respond_with(ResponseTemplate::new(status).set_body_json(body))
            .expect(1)
            .mount(&server)
            .await;
        let error = account(&server).pings().get(7).await.unwrap_err();
        assert_eq!(error.code(), code);
        assert_eq!(error.http_status(), Some(status));
    }
}

#[tokio::test]
async fn list_maps_403() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/circles.json"))
        .respond_with(
            ResponseTemplate::new(403).set_body_json(json!({"error": "Pings have been disabled"})),
        )
        .expect(1)
        .mount(&server)
        .await;
    let error = account(&server).pings().list().await.unwrap_err();
    assert_eq!(error.code(), ErrorCode::Forbidden);
    assert!(error.message().contains("Pings have been disabled"));
}

#[tokio::test]
async fn create_sends_user_ids_and_leaves_new_group_out_unless_set() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/999/circles.json"))
        .and(body_json(json!({"user_ids": [1_049_715_915_i64]})))
        .respond_with(ResponseTemplate::new(201).set_body_json(fixture(GET)))
        .expect(1)
        .mount(&server)
        .await;
    let ping = account(&server)
        .pings()
        .create(&CreatePingRequestContent {
            user_ids: vec![1_049_715_915],
            ..Default::default()
        })
        .await
        .unwrap();
    assert_eq!(ping.id, 2_085_958_600);
}

#[tokio::test]
async fn create_sends_new_group_when_set() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/999/circles.json"))
        .and(body_json(
            json!({"user_ids": [1_049_715_915_i64, 1_049_715_916_i64], "new_group": true}),
        ))
        .respond_with(ResponseTemplate::new(201).set_body_json(fixture(GET)))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .pings()
        .create(&CreatePingRequestContent {
            user_ids: vec![1_049_715_915, 1_049_715_916],
            new_group: Some(true),
        })
        .await
        .unwrap();
}

#[tokio::test]
async fn create_maps_403_and_a_bodiless_422() {
    for (status, template, code) in [
        (
            403,
            ResponseTemplate::new(403).set_body_json(json!({"error": "Pings have been disabled"})),
            ErrorCode::Forbidden,
        ),
        (422, ResponseTemplate::new(422), ErrorCode::Validation),
    ] {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .and(path("/999/circles.json"))
            .respond_with(template)
            .expect(1)
            .mount(&server)
            .await;
        let error = account(&server)
            .pings()
            .create(&CreatePingRequestContent {
                user_ids: vec![1],
                ..Default::default()
            })
            .await
            .unwrap_err();
        assert_eq!(error.code(), code);
        assert_eq!(error.http_status(), Some(status));
    }
}
