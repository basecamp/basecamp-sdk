//! SPEC §6 wire-level cases, from the bodies in `conformance/tests/error-mapping.json`.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]
#![allow(clippy::unreadable_literal)]

mod support;

use basecamp_sdk::ErrorCode;
use basecamp_sdk::models::{
    CreateProjectRequestContent, UpdateCalendarRequestContent,
    UpdateProjectClientAccessRequestContent,
};
use support::account;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

async fn error_for(status: u16, body: &str, headers: &[(&str, &str)]) -> basecamp_sdk::Error {
    let server = MockServer::start().await;
    let mut template = ResponseTemplate::new(status).set_body_string(body);
    for (name, value) in headers {
        template = template.insert_header(*name, *value);
    }
    Mock::given(method("GET"))
        .and(path("/999/projects/12345"))
        .respond_with(template)
        .expect(1)
        .mount(&server)
        .await;
    account(&server).projects().get(12345).await.unwrap_err()
}

#[tokio::test]
async fn statuses_map_to_codes_with_request_ids() {
    let cases: &[(u16, &str, ErrorCode, bool)] = &[
        (
            401,
            r#"{"error": "Unauthorized"}"#,
            ErrorCode::AuthRequired,
            false,
        ),
        (
            403,
            r#"{"error": "Forbidden"}"#,
            ErrorCode::Forbidden,
            false,
        ),
        (404, r#"{"error": "Not found"}"#, ErrorCode::NotFound, false),
        (
            400,
            r#"{"error": "Bad request"}"#,
            ErrorCode::Validation,
            false,
        ),
        (
            422,
            r#"{"error": "Name can't be blank"}"#,
            ErrorCode::Validation,
            false,
        ),
        (
            500,
            r#"{"error": "Internal server error"}"#,
            ErrorCode::ApiError,
            true,
        ),
        (
            507,
            r#"{"error": "Storage limit"}"#,
            ErrorCode::LimitExceeded,
            false,
        ),
    ];
    for (status, body, code, retryable) in cases {
        let error = error_for(*status, body, &[("X-Request-Id", "req-abc-123")]).await;
        assert_eq!(error.code(), *code, "{status}");
        assert_eq!(error.http_status(), Some(*status));
        assert_eq!(error.is_retryable(), *retryable, "{status}");
        assert_eq!(error.request_id(), Some("req-abc-123"));
        assert_eq!(error.exit_code(), code.exit_code());
    }
}

#[tokio::test]
async fn gateway_statuses_are_retryable_api_errors_but_not_retried() {
    for status in [502u16, 504] {
        let error = error_for(status, r#"{"error": "Bad Gateway"}"#, &[]).await;
        assert_eq!(error.code(), ErrorCode::ApiError);
        assert!(error.is_retryable());
        assert_eq!(error.message(), "Bad Gateway");
    }
}

#[tokio::test(start_paused = true)]
async fn rate_limits_are_retried_then_surfaced_retryable() {
    let rate_limited = || {
        support::Answer::Status(
            429,
            vec![("retry-after", "0"), ("x-request-id", "req-rate-789")],
            r#"{"error": "Rate limit exceeded"}"#,
        )
    };
    let script = support::Scripted::new(vec![
        rate_limited(),
        rate_limited(),
        rate_limited(),
        rate_limited(),
    ]);
    let config = basecamp_sdk::Config {
        max_jitter: std::time::Duration::ZERO,
        ..basecamp_sdk::Config::default()
    };
    let error = support::scripted_account(script.clone(), config)
        .projects()
        .list(&Default::default())
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::RateLimit);
    assert!(error.is_retryable());
    assert_eq!(error.retry_after(), None, "a zero Retry-After is no value");
    assert_eq!(error.request_id(), Some("req-rate-789"));
    assert_eq!(
        script.sent_count(),
        3,
        "three attempts, then the last answer is surfaced"
    );
}

#[tokio::test]
async fn field_keyed_422_bodies_flatten_into_the_message() {
    let cases: &[(&str, &str)] = &[
        (
            r#"{"errors": {"color": ["is not a valid color"]}}"#,
            "color: is not a valid color",
        ),
        (
            r#"{"errors": {"name": ["can't be blank", "is too short"], "color": ["is not a valid color"]}}"#,
            "color: is not a valid color, name: can't be blank; is too short",
        ),
        (
            r#"{"error": "Validation failed", "errors": {"color": ["is not a valid color"]}}"#,
            "Validation failed (color: is not a valid color)",
        ),
        (
            r#"{"error": {}, "errors": {"color": ["is not a valid color"]}}"#,
            "color: is not a valid color",
        ),
        (
            r#"{"message": "Validation failed", "errors": {"color": ["is not a valid color"]}}"#,
            "Validation failed (color: is not a valid color)",
        ),
        (
            r#"{"errors": {"__proto__": ["is reserved"], "color": ["is not a valid color"]}}"#,
            "__proto__: is reserved, color: is not a valid color",
        ),
        (
            r#"{"errors": {"color": ["is not a valid color"], "base": "invalid"}}"#,
            "color: is not a valid color",
        ),
    ];
    for (body, message) in cases {
        let server = MockServer::start().await;
        Mock::given(method("PUT"))
            .and(path("/999/calendars/2085958497"))
            .respond_with(ResponseTemplate::new(422).set_body_string(*body))
            .expect(1)
            .mount(&server)
            .await;
        let error = account(&server)
            .calendars()
            .update_calendar(2085958497, &UpdateCalendarRequestContent::default())
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::Validation, "{body}");
        assert_eq!(error.http_status(), Some(422));
        assert_eq!(error.message(), *message, "{body}");
        assert!(error.field_errors().is_some(), "{body}");
    }
}

#[tokio::test]
async fn bare_400_field_maps_flatten_and_flat_bodies_stay_flat() {
    let cases: &[(&str, &str, bool)] = &[
        (
            r#"{"payload_url": ["is not a valid URL"]}"#,
            "payload_url: is not a valid URL",
            true,
        ),
        (
            r#"{"types": ["is invalid"], "payload_url": ["is not a valid URL", "is too long"]}"#,
            "payload_url: is not a valid URL; is too long, types: is invalid",
            true,
        ),
        (
            r#"{"__proto__": ["is reserved"], "payload_url": ["is invalid"]}"#,
            "__proto__: is reserved, payload_url: is invalid",
            true,
        ),
        (
            r#"{"error": "Webhook is invalid", "payload_url": ["is not a valid URL"]}"#,
            "Webhook is invalid",
            false,
        ),
    ];
    for (body, message, structured) in cases {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .and(path("/999/buckets/456/webhooks.json"))
            .respond_with(ResponseTemplate::new(400).set_body_string(*body))
            .expect(1)
            .mount(&server)
            .await;
        let request = basecamp_sdk::models::CreateWebhookRequestContent {
            payload_url: "nope".to_string(),
            ..Default::default()
        };
        let error = account(&server)
            .webhooks()
            .create(456, &request)
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::Validation);
        assert_eq!(error.http_status(), Some(400));
        assert_eq!(error.message(), *message, "{body}");
        assert_eq!(error.field_errors().is_some(), *structured, "{body}");
    }
}

#[tokio::test]
async fn row_keyed_422_names_the_rejected_invitation_rows() {
    let server = MockServer::start().await;
    Mock::given(method("PUT"))
        .and(path("/999/projects/2085958505/people/client_users.json"))
        .respond_with(
            ResponseTemplate::new(422)
                .set_body_string(r#"{"errors": [{"email_address": "not-an-address", "messages": ["Email address must be valid"]}, {"email_address": null, "messages": ["Email address can't be blank"]}]}"#)
                .insert_header("Content-Type", "application/json"),
        )
        .expect(1)
        .mount(&server)
        .await;
    let error = account(&server)
        .people()
        .update_project_client_access(
            2085958505,
            &UpdateProjectClientAccessRequestContent::default(),
        )
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Validation);
    assert_eq!(error.http_status(), Some(422));
    assert_eq!(
        error.message(),
        "1: Email address can't be blank, not-an-address: Email address must be valid"
    );
    let fields = error.field_errors().unwrap();
    assert_eq!(
        fields["not-an-address"],
        vec!["Email address must be valid"]
    );
    assert_eq!(fields["1"], vec!["Email address can't be blank"]);
}

#[tokio::test]
async fn a_malformed_2xx_body_is_a_statusless_api_error_keeping_the_request_id() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/999/projects.json"))
        .respond_with(
            ResponseTemplate::new(201)
                .set_body_string(r#"{"id": "not a number"}"#)
                .insert_header("X-Request-Id", "req-decode"),
        )
        .mount(&server)
        .await;
    let request = CreateProjectRequestContent {
        name: "New".to_string(),
        ..Default::default()
    };
    let error = account(&server)
        .projects()
        .create(&request)
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::ApiError);
    assert_eq!(error.http_status(), None);
    assert!(!error.is_retryable());
    assert_eq!(error.request_id(), Some("req-decode"));
    assert!(error.message().starts_with("malformed response"));
}

#[tokio::test]
async fn an_unparseable_body_yields_the_fixed_phrase_and_keeps_the_body() {
    let error = error_for(404, "<html>gone</html>", &[]).await;
    assert_eq!(error.message(), "Request failed (HTTP 404)");
    assert_eq!(error.body(), Some(b"<html>gone</html>".as_slice()));
    assert_eq!(error.to_string(), "Request failed (HTTP 404)");
}
