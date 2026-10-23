//! Project wallpaper: the multipart upload with its optional `mode` query, the removal, and the
//! `wallpaper` object on the project read.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use basecamp_sdk::ErrorCode;
use basecamp_sdk::services::projects::UpdateProjectWallpaperParams;
use bytes::Bytes;
use serde_json::{Value, json};
use support::account;
use wiremock::matchers::{method, path, query_param, query_param_is_missing};
use wiremock::{Mock, MockServer, ResponseTemplate};

const PROJECT: &str = include_str!("../../../spec/fixtures/projects/get.json");

#[tokio::test]
async fn update_sends_the_file_as_multipart_with_the_mode() {
    let server = MockServer::start().await;
    Mock::given(method("PUT"))
        .and(path("/999/projects/42/wallpaper.json"))
        .and(query_param("mode", "tile"))
        .respond_with(ResponseTemplate::new(204))
        .expect(1)
        .mount(&server)
        .await;
    let params = UpdateProjectWallpaperParams {
        mode: Some("tile".to_string()),
    };
    account(&server)
        .projects()
        .update_project_wallpaper(
            42,
            &params,
            "beach.webp",
            "image/webp",
            Bytes::from_static(b"fake-webp-data"),
        )
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
    assert!(
        body.contains("name=\"wallpaper\"; filename=\"beach.webp\""),
        "{body}"
    );
    assert!(body.contains("fake-webp-data"), "{body}");
}

#[tokio::test]
async fn update_omits_the_mode_when_none_is_given() {
    let server = MockServer::start().await;
    Mock::given(method("PUT"))
        .and(path("/999/projects/42/wallpaper.json"))
        .and(query_param_is_missing("mode"))
        .respond_with(ResponseTemplate::new(204))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .projects()
        .update_project_wallpaper(
            42,
            &UpdateProjectWallpaperParams::default(),
            "beach.png",
            "image/png",
            Bytes::from_static(b"data"),
        )
        .await
        .unwrap();
}

#[tokio::test]
async fn update_maps_a_refused_format_to_validation() {
    let server = MockServer::start().await;
    Mock::given(method("PUT"))
        .and(path("/999/projects/42/wallpaper.json"))
        .respond_with(
            ResponseTemplate::new(422)
                .set_body_json(json!({"error": "Wallpaper is not an allowed image type"})),
        )
        .expect(1)
        .mount(&server)
        .await;
    let error = account(&server)
        .projects()
        .update_project_wallpaper(
            42,
            &UpdateProjectWallpaperParams::default(),
            "a.txt",
            "text/plain",
            Bytes::from_static(b"not an image"),
        )
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Validation);
    assert_eq!(error.http_status(), Some(422));
}

#[tokio::test]
async fn remove_deletes_the_wallpaper() {
    let server = MockServer::start().await;
    Mock::given(method("DELETE"))
        .and(path("/999/projects/42/wallpaper.json"))
        .respond_with(ResponseTemplate::new(204))
        .expect(1)
        .mount(&server)
        .await;
    account(&server)
        .projects()
        .remove_project_wallpaper(42)
        .await
        .unwrap();
}

#[tokio::test]
async fn remove_maps_a_non_editor_to_forbidden() {
    let server = MockServer::start().await;
    Mock::given(method("DELETE"))
        .and(path("/999/projects/42/wallpaper.json"))
        .respond_with(ResponseTemplate::new(403).set_body_json(json!({"error": "Access denied"})))
        .expect(1)
        .mount(&server)
        .await;
    let error = account(&server)
        .projects()
        .remove_project_wallpaper(42)
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Forbidden);
    assert_eq!(error.http_status(), Some(403));
}

#[tokio::test]
async fn get_decodes_the_wallpaper() {
    let server = MockServer::start().await;
    let fixture: Value = serde_json::from_str(PROJECT).unwrap();
    Mock::given(method("GET"))
        .and(path("/999/projects/42"))
        .respond_with(ResponseTemplate::new(200).set_body_json(&fixture))
        .expect(1)
        .mount(&server)
        .await;
    let project = account(&server).projects().get(42).await.unwrap();
    let wallpaper = project.wallpaper.unwrap();
    assert_eq!(wallpaper.url, fixture["wallpaper"]["url"]);
    assert_eq!(wallpaper.mode, "tile");
}
