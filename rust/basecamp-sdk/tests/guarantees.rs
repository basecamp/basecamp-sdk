//! Public-API guarantees: thread-safety bounds, `Send` futures, configuration validation
//! and the route table.

#![allow(clippy::unwrap_used, clippy::expect_used)]

use basecamp_sdk::{AccountClient, Client, Config, Error, ErrorCode};

fn assert_send_sync<T: Send + Sync>() {}
fn assert_clone<T: Clone>() {}
fn assert_send_future<F: std::future::Future + Send>(_: F) {}

#[test]
fn clients_and_errors_are_send_sync_and_clone() {
    assert_send_sync::<Client>();
    assert_send_sync::<AccountClient>();
    assert_send_sync::<Error>();
    assert_clone::<Client>();
    assert_clone::<AccountClient>();
    assert!(std::mem::size_of::<Error>() <= 3 * std::mem::size_of::<usize>());
}

#[test]
#[cfg(feature = "reqwest")]
fn returned_futures_are_send() {
    let account = Client::builder(Config::default())
        .access_token("t")
        .build()
        .unwrap()
        .for_account("999");
    assert_send_future(account.projects().get(1));
    assert_send_future(account.projects().list(&Default::default()));
    assert_send_future(account.download_url("https://3.basecampapi.com/999/blobs/x"));
    let collecting = account.clone();
    let page_future = async move {
        let page = collecting.projects().list(&Default::default()).await?;
        collecting.collect_all(page, None).await
    };
    assert_send_future(page_future);
    let following = account.clone();
    let next_future = async move {
        let page = following.projects().list(&Default::default()).await?;
        following.next_page(&page).await
    };
    assert_send_future(next_future);
    let streaming = account.clone();
    let stream_future = async move {
        use futures_util::StreamExt;
        let page = streaming.projects().list(&Default::default()).await?;
        let pages = streaming.pages(page).count().await;
        let page = streaming.projects().list(&Default::default()).await?;
        let items = streaming.items(page).count().await;
        Ok::<_, Error>(pages + items)
    };
    assert_send_future(stream_future);
    let composite = account.clone();
    assert_send_future(async move {
        composite
            .todos()
            .update(
                1,
                &basecamp_sdk::services::todos::UpdateTodoRequest::default(),
            )
            .await
    });
    #[cfg(feature = "oauth")]
    {
        let oauth = basecamp_sdk::OAuthClient::shipped().unwrap();
        assert_send_future(oauth.exchange_code(&basecamp_sdk::ExchangeRequest::default()));
        assert_send_future(oauth.refresh_token(&basecamp_sdk::RefreshRequest::default()));
    }
}

#[test]
#[cfg(feature = "reqwest")]
fn configuration_is_validated() {
    let error = Client::builder(Config::default().with_base_url("http://evil.example.com"))
        .access_token("t")
        .build()
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Usage);
    assert_eq!(error.message(), "base URL must use HTTPS");
    assert!(
        Client::builder(Config::default().with_base_url("http://localhost:3000"))
            .access_token("t")
            .build()
            .is_ok()
    );
    assert!(
        Client::builder(Config::default().with_base_url("http://[::1]:3000"))
            .access_token("t")
            .build()
            .is_ok()
    );
    assert!(
        Client::builder(Config::default().with_base_url("https://3.BasecampAPI.com:443/"))
            .access_token("t")
            .build()
            .is_ok()
    );

    let error = Client::builder(Config::default()).build().unwrap_err();
    assert_eq!(error.message(), "Either auth or access_token is required");
    let error = Client::builder(Config::default())
        .access_token("t")
        .access_token("u")
        .build()
        .unwrap_err();
    assert_eq!(
        error.message(),
        "Provide either auth or access_token, not both"
    );

    let config = Config {
        max_pages: 0,
        ..Config::default()
    };
    assert_eq!(
        Client::builder(config)
            .access_token("t")
            .build()
            .unwrap_err()
            .code(),
        ErrorCode::Usage
    );
    let config = Config {
        timeout: std::time::Duration::ZERO,
        ..Config::default()
    };
    assert_eq!(
        Client::builder(config)
            .access_token("t")
            .build()
            .unwrap_err()
            .code(),
        ErrorCode::Usage
    );
}

#[test]
fn the_route_table_is_complete_and_consistent() {
    use basecamp_sdk::routes::{ROUTES, Route};
    assert_eq!(ROUTES.len(), 267);
    assert_eq!(basecamp_sdk::metadata::OPERATIONS.len(), 267);
    let mut ids: Vec<&str> = ROUTES.iter().map(|route| route.id).collect();
    ids.dedup();
    assert_eq!(ids.len(), 267, "operation ids are unique");
    for route in ROUTES {
        assert_eq!(route.id, route.metadata.operation);
        assert!(route.path.starts_with('/'));
        assert!(!route.path.contains("{accountId}"));
        assert!(route.metadata.retry.max_attempts >= 1);
    }
    let idempotent = ROUTES
        .iter()
        .filter(|route| route.metadata.idempotent)
        .count();
    assert_eq!(idempotent, 91);
    let readonly = ROUTES
        .iter()
        .filter(|route| route.metadata.readonly)
        .count();
    assert_eq!(readonly, 130);
    let paginated = ROUTES
        .iter()
        .filter(|route| {
            matches!(
                route.pagination,
                basecamp_sdk::routes::Pagination::Link { .. }
            )
        })
        .count();
    assert_eq!(paginated, 61);
    let route: &Route = &basecamp_sdk::routes::GET_PROJECT;
    assert_eq!(route.fill(&[&12345]), "/projects/12345");
    assert_eq!(
        route.recognize("/projects/12345").unwrap(),
        vec![("projectId", "12345".to_string())]
    );
    assert_eq!(
        basecamp_sdk::routes::GET_PERSON_PROGRESS.pagination,
        basecamp_sdk::routes::Pagination::Link {
            key: Some("events"),
            total_count_header: Some("X-Total-Count")
        }
    );
    assert_eq!(
        basecamp_sdk::routes::REPLACE_SCHEDULE_ENTRY
            .write
            .unwrap()
            .preserved_on_omission,
        &["participant_ids", "url", "highlighted"]
    );
    assert_eq!(
        basecamp_sdk::routes::UPDATE_ACCOUNT_LOGO.body,
        basecamp_sdk::routes::BodyKind::Multipart { field: "logo" }
    );
    assert_eq!(basecamp_sdk::OPERATION_COUNT, 267);
}

#[test]
fn the_user_agent_names_the_sdk_and_the_api_version() {
    assert_eq!(
        basecamp_sdk::version::default_user_agent(),
        format!(
            "basecamp-sdk-rust/{} (api:{})",
            basecamp_sdk::VERSION,
            basecamp_sdk::API_VERSION
        )
    );
}
