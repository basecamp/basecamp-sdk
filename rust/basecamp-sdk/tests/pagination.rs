//! SPEC §8 wire-level cases, from `conformance/tests/pagination.json`.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use basecamp_sdk::services::projects::ListProjectsParams;
use basecamp_sdk::{Config, ErrorCode};
use support::{account, account_with, project};
use wiremock::matchers::{method, path, query_param};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn page(ids: &[i64], link: Option<&str>, total: Option<&str>) -> ResponseTemplate {
    let body: Vec<_> = ids.iter().map(|id| project(*id)).collect();
    let mut template = ResponseTemplate::new(200).set_body_json(body);
    if let Some(link) = link {
        template = template.insert_header("Link", link);
    }
    if let Some(total) = total {
        template = template.insert_header("X-Total-Count", total);
    }
    template
}

async fn three_pages(server: &MockServer) {
    Mock::given(method("GET"))
        .and(path("/projects.json"))
        .and(query_param("page", "3"))
        .respond_with(page(&[5], None, None))
        .mount(server)
        .await;
    Mock::given(method("GET"))
        .and(path("/projects.json"))
        .and(query_param("page", "2"))
        .respond_with(page(
            &[3, 4],
            Some("</projects.json?page=3>; rel=\"next\""),
            None,
        ))
        .mount(server)
        .await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .respond_with(page(
            &[1, 2],
            Some("</projects.json?page=2>; rel=\"next\""),
            Some("5"),
        ))
        .mount(server)
        .await;
}

#[tokio::test]
async fn the_first_page_carries_the_cursor_and_the_total() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .respond_with(page(
            &[1],
            Some("</projects.json?page=2>; rel=\"next\""),
            Some("75"),
        ))
        .expect(1)
        .mount(&server)
        .await;
    let first = account(&server)
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    assert_eq!(first.len(), 1);
    assert!(first.has_next());
    assert_eq!(first.total_count(), Some(75));
    assert_eq!(first.next_url().unwrap().path(), "/projects.json");
}

#[tokio::test]
async fn collect_all_follows_every_link() {
    let server = MockServer::start().await;
    three_pages(&server).await;
    let client = account(&server);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let all = client.collect_all(first, None).await.unwrap();
    assert_eq!(
        all.items.iter().map(|p| p.id).collect::<Vec<_>>(),
        [1, 2, 3, 4, 5]
    );
    assert_eq!(all.meta.total_count, 5);
    assert!(!all.meta.truncated);
    assert_eq!(server.received_requests().await.unwrap().len(), 3);
}

#[tokio::test]
async fn the_page_cap_truncates_and_says_so() {
    let server = MockServer::start().await;
    three_pages(&server).await;
    let config = Config {
        max_pages: 2,
        ..Config::default()
    };
    let client = account_with(&server, config);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let all = client.collect_all(first, None).await.unwrap();
    assert_eq!(all.items.len(), 4);
    assert!(all.meta.truncated);
    let next = all.meta.next_url.unwrap();
    assert_eq!(next.path(), "/projects.json");
    assert_eq!(next.query(), Some("page=3"));
    assert_eq!(server.received_requests().await.unwrap().len(), 2);
}

#[tokio::test]
async fn max_items_caps_across_pages_and_landing_exactly_is_not_truncation() {
    let server = MockServer::start().await;
    three_pages(&server).await;
    let client = account(&server);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let capped = client.collect_all(first, Some(3)).await.unwrap();
    assert_eq!(capped.items.len(), 3);
    assert!(capped.meta.truncated);

    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let exact = client.collect_all(first, Some(5)).await.unwrap();
    assert_eq!(exact.items.len(), 5);
    assert!(!exact.meta.truncated);
}

#[tokio::test]
async fn malformed_next_parts_do_not_hide_a_later_one() {
    for link in [
        "<>; rel=\"next\", </projects.json?page=2>; rel=\"next\"",
        ">x</projects.json?page=2>; rel=\"next\"",
    ] {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/projects.json"))
            .and(query_param("page", "2"))
            .respond_with(page(&[2], None, None))
            .mount(&server)
            .await;
        Mock::given(method("GET"))
            .and(path("/999/projects.json"))
            .respond_with(page(&[1], Some(link), None))
            .mount(&server)
            .await;
        let client = account(&server);
        let first = client
            .projects()
            .list(&ListProjectsParams::default())
            .await
            .unwrap();
        let all = client.collect_all(first, None).await.unwrap();
        assert_eq!(all.items.len(), 2, "{link}");
        assert!(!all.meta.truncated);
    }
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .respond_with(page(&[1], Some("<; rel=\"next\""), None))
        .expect(1)
        .mount(&server)
        .await;
    let client = account(&server);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    assert!(!first.has_next());
    assert_eq!(first.total_count(), None);
}

#[tokio::test]
async fn cross_origin_and_downgraded_links_are_refused() {
    for link in [
        "<https://evil.example.com/projects.json?page=2>; rel=\"next\"",
        "<http://3.basecampapi.com/999/projects.json?page=2>; rel=\"next\"",
    ] {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/999/projects.json"))
            .respond_with(page(&[1], Some(link), None))
            .expect(1)
            .mount(&server)
            .await;
        let client = account(&server);
        let first = client
            .projects()
            .list(&ListProjectsParams::default())
            .await
            .unwrap();
        let error = client.next_page(&first).await.unwrap_err();
        assert_eq!(error.code(), ErrorCode::Usage, "{link}");
        assert!(error.message().contains("different origin"));
        assert!(
            !error.message().contains("page=2"),
            "the target is redacted"
        );
    }
}

#[tokio::test]
async fn a_pinned_page_is_one_request_that_still_reports_truncation() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .and(query_param("page", "3"))
        .respond_with(page(
            &[5],
            Some("</projects.json?page=4>; rel=\"next\""),
            Some("9"),
        ))
        .expect(1)
        .mount(&server)
        .await;
    let client = account(&server);
    let params = ListProjectsParams {
        page: Some(3),
        ..Default::default()
    };
    let pinned = client.projects().list(&params).await.unwrap();
    assert_eq!(pinned.len(), 1);
    assert!(pinned.has_next());
    assert!(pinned.is_pinned());
    assert_eq!(pinned.total_count(), Some(9));
    assert!(
        client.next_page(&pinned).await.unwrap().is_none(),
        "a pinned page's cursor is never followed"
    );
    let all = client.collect_all(pinned, None).await.unwrap();
    assert_eq!(all.items.len(), 1);
    assert!(all.meta.truncated);
    assert_eq!(all.meta.next_url.unwrap().query(), Some("page=4"));
    assert_eq!(server.received_requests().await.unwrap().len(), 1);
}

#[tokio::test(start_paused = true)]
async fn follow_on_pages_carry_the_operation_retry_policy() {
    use support::{Answer, Scripted, scripted_account};
    let script = Scripted::new(vec![
        Answer::Status(
            200,
            vec![("link", "</999/projects.json?page=2>; rel=\"next\"")],
            "[]",
        ),
        Answer::Status(503, vec![], ""),
        Answer::Status(200, vec![], "[]"),
    ]);
    let config = Config {
        max_jitter: std::time::Duration::ZERO,
        ..Config::default()
    };
    let client = scripted_account(script.clone(), config);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let started = tokio::time::Instant::now();
    let all: basecamp_sdk::ListResult<basecamp_sdk::models::Project> =
        client.collect_all(first, None).await.unwrap();
    assert!(all.items.is_empty());
    assert_eq!(script.sent_count(), 3);
    assert_eq!(
        started.elapsed(),
        std::time::Duration::from_secs(1),
        "the second page was retried on the curve"
    );
    let sent = script.sent.lock().unwrap();
    assert_eq!(
        sent[1].uri().to_string(),
        "https://3.basecampapi.com/999/projects.json?page=2"
    );
}

#[tokio::test]
async fn follow_ups_work_on_routes_with_path_parameters() {
    use basecamp_sdk::services::todos::ListTodosParams;
    let todo = serde_json::json!({"id": 1, "status": "active", "visible_to_clients": false, "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z", "title": "x", "inherits_status": true, "type": "Todo", "url": "https://x/1.json", "app_url": "https://x/1", "bucket": {"id": 1, "name": "b", "type": "Project"}, "creator": {"id": 1, "name": "n", "email_address": "e", "personable_type": "User", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z", "admin": false, "owner": false, "client": false, "employee": false, "time_zone": "UTC", "avatar_url": "https://x/a.png"}, "parent": {"id": 2, "title": "l", "type": "Todolist", "url": "https://x/2.json", "app_url": "https://x/2"}, "content": "x", "description": "", "description_attachments": [], "completed": false, "assignees": [], "completion_subscribers": [], "completion_url": "https://x/c.json", "comments_count": 0, "comments_url": "https://x/comments.json", "bookmark_url": "https://x/b", "subscription_url": "https://x/s.json", "position": 1});
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/todolists/67890/todos.json"))
        .and(query_param("page", "2"))
        .respond_with(ResponseTemplate::new(200).set_body_json(vec![todo.clone()]))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/999/todolists/67890/todos.json"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(vec![todo])
                .insert_header("Link", "</todolists/67890/todos.json?page=2>; rel=\"next\""),
        )
        .mount(&server)
        .await;
    let client = account(&server);
    let first = client
        .todos()
        .list(67890, &ListTodosParams::default())
        .await
        .unwrap();
    let second = client
        .next_page(&first)
        .await
        .unwrap()
        .expect("a second page");
    assert_eq!(second.len(), 1);
    assert!(!second.has_next());
    let all = client
        .collect_all(
            client
                .todos()
                .list(67890, &ListTodosParams::default())
                .await
                .unwrap(),
            None,
        )
        .await
        .unwrap();
    assert_eq!(all.items.len(), 2);
}

#[tokio::test]
async fn an_unresolvable_next_target_is_an_error_not_the_end() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .respond_with(page(&[1], Some("<http://[>; rel=\"next\""), None))
        .mount(&server)
        .await;
    let client = account(&server);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    assert!(first.has_next());
    assert!(first.next_url().is_none());
    assert_eq!(
        client.next_page(&first).await.unwrap_err().code(),
        ErrorCode::Usage
    );
}

#[tokio::test]
async fn the_page_stream_is_lazy() {
    use futures_util::StreamExt;
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .respond_with(page(
            &[1],
            Some("</projects.json?page=2>; rel=\"next\""),
            None,
        ))
        .mount(&server)
        .await;
    let client = account(&server);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let mut pages = std::pin::pin!(client.pages(first));
    let page = pages.next().await.unwrap().unwrap();
    assert_eq!(page.len(), 1);
    assert_eq!(
        server.received_requests().await.unwrap().len(),
        1,
        "the successor is not fetched until asked for"
    );
    assert!(
        pages.next().await.unwrap().is_err(),
        "no mock answers page 2"
    );
}

#[tokio::test]
async fn the_page_stream_yields_a_page_before_failing_on_its_bad_cursor() {
    use futures_util::StreamExt;
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .respond_with(page(&[1], Some("<http://[>; rel=\"next\""), None))
        .mount(&server)
        .await;
    let client = account(&server);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let mut pages = std::pin::pin!(client.pages(first));
    assert_eq!(pages.next().await.unwrap().unwrap().len(), 1);
    assert_eq!(
        pages.next().await.unwrap().unwrap_err().code(),
        ErrorCode::Usage
    );
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let mut items = std::pin::pin!(client.items(first));
    assert!(items.next().await.unwrap().is_ok());
    assert!(items.next().await.unwrap().is_err());
}

#[tokio::test]
async fn follow_on_pages_report_the_same_hook_identity_as_the_first() {
    use basecamp_sdk::services::todos::ListTodosParams;
    use support::HookLog;
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/todolists/67890/todos.json"))
        .and(query_param("page", "2"))
        .respond_with(ResponseTemplate::new(200).set_body_json(Vec::<serde_json::Value>::new()))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/999/todolists/67890/todos.json"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(Vec::<serde_json::Value>::new())
                .insert_header("Link", "</todolists/67890/todos.json?page=2>; rel=\"next\""),
        )
        .mount(&server)
        .await;
    let log = std::sync::Arc::new(HookLog::default());
    let client = basecamp_sdk::Client::builder(
        Config::default()
            .with_base_url(server.uri())
            .with_timeout(std::time::Duration::from_secs(86_400)),
    )
    .access_token("test-token")
    .hooks(log.clone())
    .build()
    .unwrap()
    .for_account("999");
    let first = client
        .todos()
        .list(67890, &ListTodosParams::default())
        .await
        .unwrap();
    client.collect_all(first, None).await.unwrap();
    let starts: Vec<_> = log
        .lines()
        .into_iter()
        .filter(|line| line.starts_with("op start"))
        .collect();
    assert_eq!(
        starts,
        [
            "op start ListTodos project=None resource=Some(67890)",
            "op start ListTodos project=None resource=Some(67890)",
        ]
    );
}

#[tokio::test]
async fn the_item_stream_ends_with_an_error_when_the_page_cap_cuts_it_short() {
    use futures_util::StreamExt;
    let server = MockServer::start().await;
    three_pages(&server).await;
    let config = Config {
        max_pages: 2,
        ..Config::default()
    };
    let client = account_with(&server, config);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let mut items = std::pin::pin!(client.items(first));
    let mut ids = Vec::new();
    let mut ending = None;
    while let Some(item) = items.next().await {
        match item {
            Ok(project) => ids.push(project.id),
            Err(error) => ending = Some(error),
        }
    }
    assert_eq!(ids, [1, 2, 3, 4]);
    let ending = ending.expect("the cap is signalled");
    assert_eq!(ending.code(), ErrorCode::Usage);
    assert!(
        ending.message().contains("max_pages = 2"),
        "{}",
        ending.message()
    );
    assert_eq!(server.received_requests().await.unwrap().len(), 2);

    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let mut pages = std::pin::pin!(client.pages(first));
    assert_eq!(pages.next().await.unwrap().unwrap().len(), 2);
    let last = pages.next().await.unwrap().unwrap();
    assert!(last.has_next(), "the last page still names its successor");
    assert!(pages.next().await.is_none());

    let config = Config {
        max_pages: 3,
        ..Config::default()
    };
    let client = account_with(&server, config);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let all: Vec<_> = client.items(first).collect().await;
    assert!(
        all.iter().all(Result::is_ok),
        "a complete walk ends cleanly"
    );
    assert_eq!(all.len(), 5);
}

#[tokio::test]
async fn a_wrapped_collection_is_gathered_across_pages_from_its_envelope() {
    use basecamp_sdk::services::reports::GetPersonProgressParams;
    use futures_util::StreamExt;
    let person = serde_json::json!({"id": 45678, "name": "n", "email_address": "e", "personable_type": "User", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z", "admin": false, "owner": false, "client": false, "employee": false, "time_zone": "UTC", "avatar_url": "https://x/a.png"});
    let envelope = |ids: &[i64]| {
        let events: Vec<_> = ids.iter().map(|id| serde_json::json!({"id": id})).collect();
        serde_json::json!({"person": person.clone(), "events": events})
    };
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/reports/users/progress/45678.json"))
        .and(query_param("page", "2"))
        .respond_with(ResponseTemplate::new(200).set_body_json(envelope(&[3])))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/999/reports/users/progress/45678.json"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(envelope(&[1, 2]))
                .insert_header(
                    "Link",
                    "</reports/users/progress/45678.json?page=2>; rel=\"next\"",
                )
                .insert_header("X-Total-Count", "3"),
        )
        .mount(&server)
        .await;
    let client = account(&server);
    let first = client
        .reports()
        .person_progress(45678, &GetPersonProgressParams::default())
        .await
        .unwrap();
    assert_eq!(
        first.person.id, 45678,
        "the envelope's other members are on the page"
    );
    assert_eq!(first.events.len(), 2);
    let all = client.collect_all(first, None).await.unwrap();
    assert_eq!(
        all.items.iter().map(|event| event.id).collect::<Vec<_>>(),
        [Some(1), Some(2), Some(3)]
    );
    assert_eq!(all.meta.total_count, 3);
    assert!(!all.meta.truncated);

    let first = client
        .reports()
        .person_progress(45678, &GetPersonProgressParams::default())
        .await
        .unwrap();
    let streamed: Vec<_> = client.items(first).collect().await;
    assert_eq!(streamed.len(), 3);
    assert!(streamed.iter().all(Result::is_ok));
}

#[tokio::test]
async fn a_pinned_page_under_an_item_cap_still_reports_its_successor() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .and(query_param("page", "2"))
        .respond_with(page(
            &[3, 4],
            Some("</projects.json?page=3>; rel=\"next\""),
            None,
        ))
        .expect(1)
        .mount(&server)
        .await;
    let client = account(&server);
    let params = ListProjectsParams {
        page: Some(2),
        ..Default::default()
    };
    let pinned = client.projects().list(&params).await.unwrap();
    let all = client.collect_all(pinned, Some(2)).await.unwrap();
    assert_eq!(all.items.len(), 2);
    assert!(
        all.meta.truncated,
        "the cap was met exactly, but a page was left"
    );
    assert_eq!(all.meta.next_url.unwrap().query(), Some("page=3"));
}

#[tokio::test]
async fn a_successor_the_caps_never_reach_is_not_validated() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .respond_with(page(
            &[1, 2],
            Some("<https://evil.example.com/projects.json?page=2>; rel=\"next\""),
            None,
        ))
        .mount(&server)
        .await;
    let client = account_with(
        &server,
        Config {
            max_pages: 1,
            ..Config::default()
        },
    );
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let all = client.collect_all(first, None).await.unwrap();
    assert_eq!(all.items.len(), 2);
    assert!(all.meta.truncated);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let capped = client.collect_all(first, Some(2)).await.unwrap();
    assert!(capped.meta.truncated);
}

#[tokio::test]
async fn an_unresolvable_cursor_past_the_item_cap_is_not_an_error() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/999/projects.json"))
        .respond_with(page(&[1, 2], Some("<http://[>; rel=\"next\""), None))
        .mount(&server)
        .await;
    let client = account(&server);
    let first = client
        .projects()
        .list(&ListProjectsParams::default())
        .await
        .unwrap();
    let capped = client.collect_all(first, Some(2)).await.unwrap();
    assert_eq!(capped.items.len(), 2);
    assert!(capped.meta.truncated);
}
