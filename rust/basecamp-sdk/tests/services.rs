//! One generated call per service: the accessor, the method, the path, the account prefix and
//! the SPEC §13 headers reach the wire. One representative operation per service, chosen as the
//! simplest read the service offers.

#![cfg(feature = "reqwest")]

mod support;

use support::account;
use wiremock::matchers::{header, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

#[tokio::test]
async fn account_account_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/account.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"created_at": "2025-01-01T00:00:00Z", "id": 1, "name": "x", "updated_at": "2025-01-01T00:00:00Z"})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).account().account().await.unwrap();
}

#[tokio::test]
async fn attachments_create_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/999/attachments.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(201).set_body_json(serde_json::json!({})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .attachments()
        .create(
            "q",
            "application/octet-stream",
            bytes::Bytes::from_static(b"x"),
        )
        .await
        .unwrap();
}

#[tokio::test]
async fn automation_list_lineup_markers_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/lineup/markers.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .automation()
        .list_lineup_markers()
        .await
        .unwrap();
}

#[tokio::test]
async fn bookmarks_list_my_bookmarks_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/my/bookmarks.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .bookmarks()
        .list_my_bookmarks(&Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn boosts_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/boosts/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(serde_json::json!({"created_at": "2025-01-01T00:00:00Z", "id": 1})),
        )
        .expect(1)
        .mount(&server)
        .await;
    account(&server).boosts().get(100).await.unwrap();
}

#[tokio::test]
async fn bubble_ups_delete_bubble_up_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("DELETE"))
        .and(path("/999/recordings/100/bubble_up.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(204))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .bubble_ups()
        .delete_bubble_up(100)
        .await
        .unwrap();
}

#[tokio::test]
async fn calendars_get_calendar_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/calendars/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "color": "x", "created_at": "2025-01-01T00:00:00Z", "id": 1, "name": "x", "schedule_url": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x"})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .calendars()
        .get_calendar(100)
        .await
        .unwrap();
}

#[tokio::test]
async fn campfires_list_chatbots_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/buckets/100/chats/101/integrations.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .campfires()
        .list_chatbots(100, 101)
        .await
        .unwrap();
}

#[tokio::test]
async fn card_columns_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/card_tables/columns/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).card_columns().get(100).await.unwrap();
}

#[tokio::test]
async fn card_steps_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/card_tables/steps/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).card_steps().get(100).await.unwrap();
}

#[tokio::test]
async fn card_tables_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/card_tables/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).card_tables().get(100).await.unwrap();
}

#[tokio::test]
async fn cards_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/card_tables/cards/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "description_attachments": [], "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).cards().get(100).await.unwrap();
}

#[tokio::test]
async fn checkins_reminders_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/my/question_reminders.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .checkins()
        .reminders(&Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn client_approvals_list_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/buckets/100/client/approvals.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .client_approvals()
        .list(100, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn client_correspondences_list_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/buckets/100/client/correspondences.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .client_correspondences()
        .list(100, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn client_replies_list_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/buckets/100/client/recordings/101/replies.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .client_replies()
        .list(100, 101, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn client_visibility_set_visibility_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("PUT"))
        .and(path("/999/recordings/100/client_visibility.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .client_visibility()
        .set_visibility(100, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn cloud_files_cloud_file_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/cloud_files/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "description_attachments": [], "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "service": {"code": "x", "example_url": "x", "name": "x", "valid_patterns": []}, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .cloud_files()
        .cloud_file(100)
        .await
        .unwrap();
}

#[tokio::test]
async fn comments_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/comments/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "content": "x", "content_attachments": [], "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).comments().get(100).await.unwrap();
}

#[tokio::test]
async fn documents_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/documents/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "content_attachments": [], "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).documents().get(100).await.unwrap();
}

#[tokio::test]
async fn drafts_list_my_drafts_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/my/drafts.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .drafts()
        .list_my_drafts(&Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn events_list_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/recordings/100/events.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .events()
        .list(100, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn everything_everything_completed_cards_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/cards/completed.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .everything()
        .everything_completed_cards(&Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn folders_list_folders_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/stacks.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).folders().list_folders().await.unwrap();
}

#[tokio::test]
async fn forwards_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/inbox_forwards/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "content_attachments": [], "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "status": "x", "subject": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).forwards().get(100).await.unwrap();
}

#[tokio::test]
async fn gauges_gauge_needle_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/gauge_needles/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"created_at": "2025-01-01T00:00:00Z", "description_attachments": [], "id": 1, "updated_at": "2025-01-01T00:00:00Z"})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).gauges().gauge_needle(100).await.unwrap();
}

#[tokio::test]
async fn google_documents_google_document_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/google_documents/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "description_attachments": [], "document_type": "x", "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .google_documents()
        .google_document(100)
        .await
        .unwrap();
}

#[tokio::test]
async fn hill_charts_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/todosets/100/hill.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(serde_json::json!({"enabled": false, "stale": false})),
        )
        .expect(1)
        .mount(&server)
        .await;
    account(&server).hill_charts().get(100).await.unwrap();
}

#[tokio::test]
async fn lineup_delete_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("DELETE"))
        .and(path("/999/lineup/markers/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(204))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).lineup().delete(100).await.unwrap();
}

#[tokio::test]
async fn message_boards_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/message_boards/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).message_boards().get(100).await.unwrap();
}

#[tokio::test]
async fn message_types_list_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/buckets/100/categories.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).message_types().list(100).await.unwrap();
}

#[tokio::test]
async fn messages_list_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/message_boards/100/messages.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .messages()
        .list(100, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn my_assignments_my_assignments_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/my/assignments.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .my_assignments()
        .my_assignments()
        .await
        .unwrap();
}

#[tokio::test]
async fn my_notes_get_my_note_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/my/notes.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "content": "x", "content_attachments": [], "created_at": null, "id": null, "type": "x", "updated_at": null, "url": "x"})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).my_notes().get_my_note().await.unwrap();
}

#[tokio::test]
async fn my_notifications_my_notifications_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/my/readings.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(
            serde_json::json!({"bubble_ups_count": 1, "scheduled_bubble_ups_count": 1}),
        ))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .my_notifications()
        .my_notifications(&Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn people_list_pingable_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/circles/people.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).people().list_pingable().await.unwrap();
}

#[tokio::test]
async fn projects_list_recent_projects_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/my/recent_projects.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .projects()
        .list_recent_projects()
        .await
        .unwrap();
}

#[tokio::test]
async fn recordings_unspotlight_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("DELETE"))
        .and(path("/999/recordings/100/spotlight.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(204))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .recordings()
        .unspotlight(100)
        .await
        .unwrap();
}

#[tokio::test]
async fn reports_progress_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/reports/progress.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .reports()
        .progress(&Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn schedules_get_entry_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/schedule_entries/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"all_day": false, "app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "description_attachments": [], "ends_at": "x", "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "starts_at": "x", "status": "x", "summary": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).schedules().get_entry(100).await.unwrap();
}

#[tokio::test]
async fn search_metadata_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/searches/metadata.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"default_bucket_label": "x", "default_circle_label": "x", "default_creator_label": "x", "default_file_type_label": "x", "default_type_label": "x", "file_search_types": [], "recording_search_types": []})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).search().metadata().await.unwrap();
}

#[tokio::test]
async fn subscriptions_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/recordings/100/subscription.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(serde_json::json!({"count": 1, "subscribed": false, "url": "x"})),
        )
        .expect(1)
        .mount(&server)
        .await;
    account(&server).subscriptions().get(100).await.unwrap();
}

#[tokio::test]
async fn templates_get_library_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/template_library.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"bucket": {"id": 1, "name": "x", "type": "x"}, "todolists": [], "todoset": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).templates().get_library().await.unwrap();
}

#[tokio::test]
async fn timeline_project_timeline_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects/100/timeline.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .timeline()
        .project_timeline(100, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn timesheets_for_project_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects/100/timesheet.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .timesheets()
        .for_project(100, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn todolist_groups_list_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/todolists/100/groups.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .todolist_groups()
        .list(100, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn todolists_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/todolists/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bubble_up_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "comments_app_url": "x", "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "description": "x", "description_attachments": [], "id": 1, "inherits_status": false, "name": "x", "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false, "color": null})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).todolists().get(100).await.unwrap();
}

#[tokio::test]
async fn todos_list_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/todolists/100/todos.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .todos()
        .list(100, &Default::default())
        .await
        .unwrap();
}

#[tokio::test]
async fn todosets_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/todosets/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "name": "x", "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).todosets().get(100).await.unwrap();
}

#[tokio::test]
async fn tools_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/dock/tools/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).tools().get(100).await.unwrap();
}

#[tokio::test]
async fn uploads_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/uploads/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "description_attachments": [], "id": 1, "inherits_status": false, "parent": {"app_url": "x", "id": 1, "title": "x", "type": "x", "url": "x"}, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).uploads().get(100).await.unwrap();
}

#[tokio::test]
async fn vaults_get_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/vaults/100"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header("User-Agent", basecamp_sdk::version::default_user_agent().as_str()))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"app_url": "x", "bucket": {"id": 1, "name": "x", "type": "x"}, "created_at": "2025-01-01T00:00:00Z", "creator": {"id": 1, "name": "x"}, "id": 1, "inherits_status": false, "status": "x", "title": "x", "type": "x", "updated_at": "2025-01-01T00:00:00Z", "url": "x", "visible_to_clients": false})))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).vaults().get(100).await.unwrap();
}

#[tokio::test]
async fn webhooks_list_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/buckets/100/webhooks.json"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).webhooks().list(100).await.unwrap();
}

#[tokio::test]
async fn wormholes_delete_reaches_the_wire() {
    let server = MockServer::start().await;
    Mock::given(method("DELETE"))
        .and(path("/999/buckets/100/card_tables/wormholes/101"))
        .and(header("Authorization", "Bearer test-token"))
        .and(header("Accept", "application/json"))
        .and(header(
            "User-Agent",
            basecamp_sdk::version::default_user_agent().as_str(),
        ))
        .respond_with(ResponseTemplate::new(204))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).wormholes().delete(100, 101).await.unwrap();
}

#[tokio::test]
async fn a_base_url_path_prefix_is_kept_in_front_of_the_account() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/v1/999/account.json"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"created_at": "2025-01-01T00:00:00Z", "id": 1, "name": "x", "updated_at": "2025-01-01T00:00:00Z"})))
        .expect(1)
        .mount(&server)
        .await;
    let account = basecamp_sdk::Client::builder(
        basecamp_sdk::Config::default().with_base_url(format!("{}/api/v1/", server.uri())),
    )
    .access_token("test-token")
    .build()
    .unwrap()
    .for_account("999");
    account.account().account().await.unwrap();
}

#[tokio::test]
async fn an_account_id_is_one_path_segment_or_a_usage_error() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/a%20b/account.json"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"created_at": "2025-01-01T00:00:00Z", "id": 1, "name": "x", "updated_at": "2025-01-01T00:00:00Z"})))
        .expect(1)
        .mount(&server)
        .await;
    let client =
        basecamp_sdk::Client::builder(basecamp_sdk::Config::default().with_base_url(server.uri()))
            .access_token("test-token")
            .build()
            .unwrap();
    client.for_account("a b").account().account().await.unwrap();
    for account in ["", ".", ".."] {
        let error = client
            .for_account(account)
            .account()
            .account()
            .await
            .unwrap_err();
        assert_eq!(error.code(), basecamp_sdk::ErrorCode::Usage, "{account:?}");
    }
    assert_eq!(server.received_requests().await.unwrap().len(), 1);
    let error = client
        .for_account("../../evil")
        .account()
        .account()
        .await
        .unwrap_err();
    assert_eq!(error.code(), basecamp_sdk::ErrorCode::NotFound);
    let requests = server.received_requests().await.unwrap();
    assert_eq!(
        requests[1].url.path(),
        "/..%2F..%2Fevil/account.json",
        "a path-shaped id stays one segment"
    );
}
