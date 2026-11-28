//! The current user's avatar: the multipart upload and the removal.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use basecamp_sdk::ErrorCode;
use bytes::Bytes;
use serde_json::json;
use support::account;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

#[tokio::test]
async fn update_sends_one_avatar_file_part() {
    let server = MockServer::start().await;
    Mock::given(method("PUT"))
        .and(path("/999/my/avatar.json"))
        .respond_with(ResponseTemplate::new(204))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .people()
        .update_my_avatar("me.png", "image/png", Bytes::from_static(b"avatar-bytes"))
        .await
        .unwrap();

    let requests = server.received_requests().await.unwrap();
    let content_type = requests[0].headers.get("content-type").unwrap();
    assert!(
        content_type
            .to_str()
            .unwrap()
            .starts_with("multipart/form-data; boundary="),
        "{content_type:?}"
    );
    let body = String::from_utf8_lossy(&requests[0].body);
    assert_eq!(
        body.matches("Content-Disposition: form-data").count(),
        1,
        "{body}"
    );
    assert!(
        body.contains(
            "Content-Disposition: form-data; name=\"avatar\"; filename=\"me.png\"\r\n\
             Content-Type: image/png\r\n\r\navatar-bytes\r\n"
        ),
        "{body}"
    );
}

#[tokio::test]
async fn update_maps_a_refused_image_to_validation() {
    let server = MockServer::start().await;
    Mock::given(method("PUT"))
        .and(path("/999/my/avatar.json"))
        .respond_with(
            ResponseTemplate::new(422)
                .set_body_json(json!({"error": "Avatar is not an allowed image type"})),
        )
        .expect(1)
        .mount(&server)
        .await;
    let error = account(&server)
        .people()
        .update_my_avatar("a.txt", "text/plain", Bytes::from_static(b"not an image"))
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Validation);
    assert_eq!(error.http_status(), Some(422));
}

#[tokio::test]
async fn delete_removes_the_avatar() {
    let server = MockServer::start().await;
    Mock::given(method("DELETE"))
        .and(path("/999/my/avatar.json"))
        .respond_with(ResponseTemplate::new(204))
        .expect(1)
        .mount(&server)
        .await;
    account(&server).people().delete_my_avatar().await.unwrap();
}

#[tokio::test]
async fn delete_maps_a_refusal_to_forbidden() {
    let server = MockServer::start().await;
    Mock::given(method("DELETE"))
        .and(path("/999/my/avatar.json"))
        .respond_with(ResponseTemplate::new(403).set_body_json(json!({"error": "Access denied"})))
        .expect(1)
        .mount(&server)
        .await;
    let error = account(&server)
        .people()
        .delete_my_avatar()
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Forbidden);
    assert_eq!(error.http_status(), Some(403));
}
