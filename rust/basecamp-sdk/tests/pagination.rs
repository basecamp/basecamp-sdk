//! SPEC §8 wire-level cases, from `conformance/tests/pagination.json`.

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
    let mut config = Config::default();
    config.max_pages = 2;
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
    assert_eq!(pinned.total_count(), Some(9));
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
    let mut config = Config::default();
    config.max_jitter = std::time::Duration::ZERO;
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
