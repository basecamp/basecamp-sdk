//! SPEC §18 / Appendix F: the recording-summary projection, its Campfire discovery, and the
//! mention composites.
//!
//! `conformance/tests/recording_summary.json` is the contract, and the Rust conformance
//! runner executes all 40 of its cases. What is here is what a single-case fixture cannot
//! reach: the discovery index living ACROSS calls, the budget and listing bounds, the
//! difference between "unresolved" and "incomplete", and the hook identities SPEC §18 rule 3
//! requires.

#![allow(clippy::unwrap_used, clippy::expect_used)]
#![cfg(feature = "reqwest")]

mod support;

use std::sync::Arc;

use basecamp_sdk::services::campfire_index::MAX_CAMPFIRE_CANDIDATES;
use basecamp_sdk::services::recordings::{
    RecordingRef, RecordingSummary, RecordingSummaryError, summarizable_event_types,
    summarizable_recording_types,
};
use basecamp_sdk::{AccountClient, Client, Config, Error, ErrorCode};
use serde_json::{Value, json};
use support::HookLog;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

const BUCKET: i64 = 2_085_958_499;
const OTHER_BUCKET: i64 = 2_085_958_500;
const LINE: i64 = 1_069_479_350;
const FIRST_CAMPFIRE: i64 = 1_069_479_340;
const SECOND_CAMPFIRE: i64 = 1_069_479_345;
const ANNIE: i64 = 1_049_715_915;
/// Annie's `attachable_sgid`: `{"gid" => "gid://bc3/Person/1049715915?expires_in",
/// "purpose" => "attachable", "expires_at" => nil}`, Marshal 4.8, as BC3 mints it.
const ANNIE_SGID: &str = "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102";

fn person(id: i64, sgid: Option<&str>) -> Value {
    let mut person = json!({ "id": id, "name": "Victor Cooper" });
    if let Some(sgid) = sgid {
        person["attachable_sgid"] = json!(sgid);
    }
    person
}

fn campfire(id: i64, bucket: i64) -> Value {
    json!({
        "id": id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2022-10-28T15:25:00.000Z",
        "updated_at": "2022-10-28T15:25:00.000Z",
        "title": "Campfire",
        "inherits_status": true,
        "type": "Chat::Transcript",
        "url": "https://3.basecampapi.com/999/buckets/x/chats/x.json",
        "app_url": "https://3.basecamp.com/999/buckets/x/chats/x",
        "bucket": { "id": bucket, "name": "A project", "type": "Project" },
        "creator": person(1, None),
    })
}

fn chat_line(bucket: i64, campfire_id: i64, line_type: &str, content: &str) -> Value {
    json!({
        "id": LINE,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2022-10-28T15:25:00.000Z",
        "updated_at": "2022-10-28T15:25:00.000Z",
        "title": "Hello everyone!",
        "inherits_status": true,
        "type": line_type,
        "url": "https://3.basecampapi.com/999/buckets/x/chats/x/lines/x.json",
        "app_url": "https://3.basecamp.com/999/buckets/x/chats/x",
        "content": content,
        "parent": {
            "id": campfire_id,
            "title": "Campfire",
            "type": "Chat::Transcript",
            "url": "https://3.basecampapi.com/999/buckets/x/chats/x.json",
            "app_url": "https://3.basecamp.com/999/buckets/x/chats/x",
        },
        "bucket": { "id": bucket, "name": "A project", "type": "Project" },
        "creator": person(1, None),
    })
}

fn comment(bucket: i64, content: &str) -> Value {
    json!({
        "id": 1_069_479_361_i64,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2022-10-30T01:01:58.169Z",
        "updated_at": "2022-10-30T01:01:58.169Z",
        "title": "Re: We won Leto!",
        "inherits_status": true,
        "type": "Comment",
        "url": "https://3.basecampapi.com/999/buckets/x/comments/x.json",
        "app_url": "https://3.basecamp.com/999/buckets/x/comments/x",
        "parent": {
            "id": 1_069_479_351_i64,
            "title": "We won Leto!",
            "type": "Message",
            "url": "https://3.basecampapi.com/999/buckets/x/messages/x.json",
            "app_url": "https://3.basecamp.com/999/buckets/x/messages/x",
        },
        "bucket": { "id": bucket, "name": "A project", "type": "Project" },
        "creator": person(ANNIE, Some(ANNIE_SGID)),
        "content": content,
        "content_attachments": [],
    })
}

fn project_with_chat(campfire_id: i64) -> Value {
    json!({
        "id": BUCKET,
        "status": "active",
        "created_at": "2022-10-28T15:25:00.000Z",
        "updated_at": "2022-10-28T15:25:00.000Z",
        "name": "A project",
        "url": "https://3.basecampapi.com/999/projects/x.json",
        "app_url": "https://3.basecamp.com/999/projects/x",
        "dock": [
            { "id": 1, "title": "To-dos", "name": "todoset", "enabled": true, "url": "u", "app_url": "a" },
            { "id": campfire_id, "title": "Campfire", "name": "chat", "enabled": true, "url": "u", "app_url": "a" },
        ],
    })
}

async fn mount(server: &MockServer, verb: &str, route: &str, status: u16, body: &Value) {
    Mock::given(method(verb))
        .and(path(route))
        .respond_with(ResponseTemplate::new(status).set_body_json(body.clone()))
        .mount(server)
        .await;
}

fn not_found() -> Value {
    json!({ "error": "Record not found" })
}

fn account(server: &MockServer) -> AccountClient {
    support::account(server)
}

async fn paths(server: &MockServer) -> Vec<String> {
    server
        .received_requests()
        .await
        .unwrap_or_default()
        .iter()
        .map(|request| request.url.path().to_string())
        .collect()
}

fn line_path(campfire_id: i64) -> String {
    format!("/999/chats/{campfire_id}/lines/{LINE}")
}

/// The dock, the listing and both line reads, as the fixture's discovery case scripts them:
/// the bucket is not a project, so the listing supplies two candidates and the second
/// answers.
async fn discovery_server(second_line_status: u16) -> MockServer {
    let server = MockServer::start().await;
    mount(
        &server,
        "GET",
        &format!("/999/projects/{BUCKET}"),
        404,
        &not_found(),
    )
    .await;
    mount(
        &server,
        "GET",
        "/999/chats.json",
        200,
        &json!([
            campfire(1_069_479_400, OTHER_BUCKET),
            campfire(FIRST_CAMPFIRE, BUCKET),
            campfire(SECOND_CAMPFIRE, BUCKET),
        ]),
    )
    .await;
    mount(
        &server,
        "GET",
        &line_path(FIRST_CAMPFIRE),
        404,
        &not_found(),
    )
    .await;
    mount(
        &server,
        "GET",
        &line_path(SECOND_CAMPFIRE),
        second_line_status,
        &if second_line_status == 200 {
            chat_line(BUCKET, SECOND_CAMPFIRE, "Chat::Lines::Text", "Hello!")
        } else {
            not_found()
        },
    )
    .await;
    server
}

fn chat_line_ref() -> RecordingRef {
    RecordingRef::from_event(BUCKET, LINE, "chat.line.created")
}

#[tokio::test]
async fn the_discovery_index_is_reused_across_calls_on_one_client() {
    let server = discovery_server(200).await;
    let account = account(&server);
    let first = account.recordings().summarize(&chat_line_ref()).await;
    assert_eq!(first.unwrap().campfire_id, Some(SECOND_CAMPFIRE));
    assert_eq!(paths(&server).await.len(), 4);

    // The dock and the listing are both cached now, so the second call pays only for the
    // line reads: nothing re-reads a source inside its TTL.
    let second = account.recordings().summarize(&chat_line_ref()).await;
    assert_eq!(second.unwrap().campfire_id, Some(SECOND_CAMPFIRE));
    let paths = paths(&server).await;
    assert_eq!(paths.len(), 6);
    assert_eq!(
        &paths[4..],
        &[line_path(FIRST_CAMPFIRE), line_path(SECOND_CAMPFIRE)]
    );
}

#[tokio::test]
async fn the_index_is_shared_by_every_account_client_of_one_client() {
    let server = discovery_server(200).await;
    let client = Client::builder(Config::default().with_base_url(server.uri()))
        .access_token("test-token")
        .build()
        .unwrap();
    client
        .for_account("999")
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap();
    client
        .for_account("999")
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap();
    assert_eq!(paths(&server).await.len(), 6, "one dock and one listing");
}

#[tokio::test]
async fn a_separate_client_starts_with_an_empty_index() {
    let server = discovery_server(200).await;
    for _ in 0..2 {
        account(&server)
            .recordings()
            .summarize(&chat_line_ref())
            .await
            .unwrap();
    }
    assert_eq!(
        paths(&server).await.len(),
        8,
        "a second client re-reads both sources"
    );
}

/// The dock has its say before the listing is fetched, so a listing that is down never
/// stands between a project's line and the one project read that finds it. The listing is
/// mounted as a 503 here rather than left unmounted: an unmounted route proves only that
/// the request was not made, while a failing one proves the call does not depend on it.
#[tokio::test]
async fn the_dock_is_read_first_and_the_listing_only_when_it_misses() {
    let server = MockServer::start().await;
    mount(
        &server,
        "GET",
        &format!("/999/projects/{BUCKET}"),
        200,
        &project_with_chat(SECOND_CAMPFIRE),
    )
    .await;
    mount(
        &server,
        "GET",
        "/999/chats.json",
        503,
        &json!({ "error": "the listing is down" }),
    )
    .await;
    mount(
        &server,
        "GET",
        &line_path(SECOND_CAMPFIRE),
        200,
        &chat_line(BUCKET, SECOND_CAMPFIRE, "Chat::Lines::Text", "Hello!"),
    )
    .await;
    let summary = account(&server)
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap();
    assert_eq!(summary.campfire_id, Some(SECOND_CAMPFIRE));
    assert_eq!(
        paths(&server).await,
        vec![
            format!("/999/projects/{BUCKET}"),
            line_path(SECOND_CAMPFIRE)
        ],
        "the expensive account-wide listing is never fetched"
    );
}

#[tokio::test]
async fn a_line_under_no_visible_campfire_is_unresolved_not_a_failed_read() {
    let server = discovery_server(404).await;
    let error = account(&server)
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    let Some(RecordingSummaryError::Unresolved(unresolved)) = RecordingSummaryError::of(&error)
    else {
        panic!("expected an unresolved line, got {error}");
    };
    assert_eq!(
        unresolved.campfire_ids,
        vec![FIRST_CAMPFIRE, SECOND_CAMPFIRE]
    );
    assert_eq!(unresolved.bucket_id, BUCKET);
    assert_eq!(unresolved.recording_id, LINE);
    // Both sources were loaded during this call, so there was nothing cached to refresh.
    assert!(!unresolved.refreshed);
    assert!(unresolved.stale_campfire_ids.is_empty());
    assert_eq!(error.code(), ErrorCode::NotFound);
    assert!(
        error
            .to_string()
            .contains("found under no visible campfire")
    );
}

#[tokio::test]
async fn a_second_unresolvable_line_declines_the_refresh_under_the_floor() {
    let server = discovery_server(404).await;
    let account = account(&server);
    account
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    let error = account
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    let Some(RecordingSummaryError::Unresolved(unresolved)) = RecordingSummaryError::of(&error)
    else {
        panic!("expected an unresolved line, got {error}");
    };
    // The floor declined both re-reads, so the conclusion stands on cached data and says so
    // — and a run of unresolvable lines never turns into a listing per line.
    assert!(!unresolved.refreshed);
    assert_eq!(
        paths(&server).await.len(),
        6,
        "only the two line reads were paid for again"
    );
}

#[tokio::test]
async fn a_candidates_non_404_answer_is_returned_as_that_reads_own_error() {
    let server = discovery_server(403).await;
    let error = account(&server)
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Forbidden);
    assert_eq!(error.http_status(), Some(403));
    assert!(
        RecordingSummaryError::of(&error).is_none(),
        "a permission failure is not the composite's own verdict"
    );
    assert_eq!(paths(&server).await.len(), 4);
}

#[tokio::test]
async fn a_dock_read_that_fails_for_any_other_reason_stops_the_search() {
    let server = MockServer::start().await;
    mount(
        &server,
        "GET",
        &format!("/999/projects/{BUCKET}"),
        403,
        &json!({ "error": "forbidden" }),
    )
    .await;
    let error = account(&server)
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Forbidden);
    assert_eq!(paths(&server).await.len(), 1, "no listing, no line reads");
}

#[tokio::test]
async fn more_candidates_than_the_budget_is_incomplete_discovery_never_unresolved() {
    let server = MockServer::start().await;
    mount(
        &server,
        "GET",
        &format!("/999/projects/{BUCKET}"),
        404,
        &not_found(),
    )
    .await;
    let crowded: Vec<Value> = (0..=MAX_CAMPFIRE_CANDIDATES)
        .map(|index| campfire(9_000_000 + i64::try_from(index).unwrap(), BUCKET))
        .collect();
    mount(&server, "GET", "/999/chats.json", 200, &json!(crowded)).await;
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(404).set_body_json(not_found()))
        .mount(&server)
        .await;
    let error = account(&server)
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    let Some(RecordingSummaryError::CampfireDiscoveryIncomplete { reason, .. }) =
        RecordingSummaryError::of(&error)
    else {
        panic!("expected incomplete discovery, got {error}");
    };
    assert!(reason.contains(&MAX_CAMPFIRE_CANDIDATES.to_string()));
    // Never not_found: nothing left unsearched may be reported absent. `usage`
    // is one of only three coarse codes no HTTP response can produce (with
    // `network` and `ambiguous`), and the one of those three that also describes
    // a call the SDK declined to complete, so this verdict can never be read back
    // as a constituent read's own answer — settled for every port on card 40
    // after this one shipped `api_error` and Kotlin `usage`.
    // Non-retryable is the other half of that decision.
    assert_eq!(error.code(), ErrorCode::Usage);
    assert!(!error.is_retryable());
}

/// The boundary the `skipped` flag cannot express: a dock holding EXACTLY the budget, every
/// candidate answering 404. The budget is spent but nothing was ever passed over, so
/// `skipped` is false — and the account listing has not been consulted, so candidates may
/// exist there unsearched. That is `incomplete`, and it must not be a listing fetch: the
/// fetch could admit no candidate, and a failure on it would replace a settled verdict with
/// a transient error the consumer retries forever.
#[tokio::test]
async fn a_budget_spent_before_the_listing_is_incomplete_and_costs_no_listing_fetch() {
    let server = MockServer::start().await;
    let crowded: Vec<Value> = (0..MAX_CAMPFIRE_CANDIDATES)
        .map(|index| {
            json!({
                "id": 9_000_000 + i64::try_from(index).unwrap(),
                "title": "Campfire",
                "name": "chat",
                "enabled": true,
                "url": "u",
                "app_url": "a",
            })
        })
        .collect();
    let mut project = project_with_chat(SECOND_CAMPFIRE);
    project["dock"] = json!(crowded);
    mount(
        &server,
        "GET",
        &format!("/999/projects/{BUCKET}"),
        200,
        &project,
    )
    .await;
    // Every line read answers "not here".
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(404).set_body_json(not_found()))
        .mount(&server)
        .await;

    let error = account(&server)
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    let Some(RecordingSummaryError::CampfireDiscoveryIncomplete { reason, .. }) =
        RecordingSummaryError::of(&error)
    else {
        panic!("expected incomplete discovery, got {error}");
    };
    assert!(
        reason.contains("before the account listing was consulted"),
        "the reason names the source that went unsearched: {reason}"
    );
    let paths = paths(&server).await;
    assert_eq!(
        paths.len(),
        1 + MAX_CAMPFIRE_CANDIDATES,
        "one dock read and the whole budget, and nothing else"
    );
    assert!(
        !paths.iter().any(|path| path == "/999/chats.json"),
        "a listing fetch that could admit no candidate was not paid for"
    );
}

/// The other half of the same rule: both sources WERE consulted and the budget ran out, so
/// nothing went unsearched and the answer is the settled `unresolved`, not `incomplete`. The
/// re-reads are skipped — they could hand this call no candidate it may try — and the
/// conclusion says so by reporting `refreshed` false.
///
/// The bucket holds exactly `MAX_CAMPFIRE_CANDIDATES` campfires so the budget is genuinely
/// spent. An earlier version of this test used a two-campfire bucket and finished with 48
/// of the budget left: it passed because of the refresh floor, not because of the rule it
/// claimed to pin, and the named property went untested.
#[tokio::test]
async fn a_budget_spent_after_both_sources_were_consulted_is_unresolved_not_incomplete() {
    let server = MockServer::start().await;
    let ids: Vec<i64> = (0..MAX_CAMPFIRE_CANDIDATES)
        .map(|index| 9_000_000 + i64::try_from(index).unwrap())
        .collect();
    mount(
        &server,
        "GET",
        &format!("/999/projects/{BUCKET}"),
        404,
        &not_found(),
    )
    .await;
    mount(
        &server,
        "GET",
        "/999/chats.json",
        200,
        &json!(
            ids.iter()
                .map(|id| campfire(*id, BUCKET))
                .collect::<Vec<_>>()
        ),
    )
    .await;
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(404).set_body_json(not_found()))
        .mount(&server)
        .await;

    let account = account(&server);
    // First call fills both caches and spends the budget on the listing's candidates.
    account
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    let after_first = paths(&server).await.len();

    // Second call: both sources are consulted from cache in pass 1 and the budget is spent
    // there, so pass 2 re-reads neither and the verdict stands on what was seen.
    let error = account
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    let Some(RecordingSummaryError::Unresolved(unresolved)) = RecordingSummaryError::of(&error)
    else {
        panic!("expected unresolved, got {error}");
    };
    assert!(!unresolved.refreshed);
    assert_eq!(unresolved.campfire_ids, ids);
    assert!(
        unresolved.stale_campfire_ids.is_empty(),
        "nothing changed, so nothing is stale"
    );
    assert_eq!(
        paths(&server).await.len() - after_first,
        MAX_CAMPFIRE_CANDIDATES,
        "the budget's worth of line reads; neither source was re-read"
    );
}

/// Only a listing OVER ITS CAP is incomplete discovery. Any other listing failure is that
/// read's own error and passes through: a 403 is not a settled verdict about where the line
/// is, and reporting it as one would tell a consumer to stop looking.
#[tokio::test]
async fn a_listing_failure_that_is_not_an_overflow_passes_through() {
    for (status, code) in [
        (403, ErrorCode::Forbidden),
        (401, ErrorCode::AuthRequired),
        (422, ErrorCode::Validation),
    ] {
        let server = MockServer::start().await;
        mount(
            &server,
            "GET",
            &format!("/999/projects/{BUCKET}"),
            404,
            &not_found(),
        )
        .await;
        mount(
            &server,
            "GET",
            "/999/chats.json",
            status,
            &json!({ "error": "nope" }),
        )
        .await;
        let error = account(&server)
            .recordings()
            .summarize(&chat_line_ref())
            .await
            .unwrap_err();
        assert_eq!(error.code(), code, "status {status}");
        assert_eq!(error.http_status(), Some(status));
        assert!(
            RecordingSummaryError::of(&error).is_none(),
            "a failed listing read is not the composite's own verdict (status {status})"
        );
    }
}

#[tokio::test]
async fn a_listing_that_overflows_its_cap_is_incomplete_discovery_too() {
    let server = MockServer::start().await;
    mount(
        &server,
        "GET",
        &format!("/999/projects/{BUCKET}"),
        404,
        &not_found(),
    )
    .await;
    Mock::given(method("GET"))
        .and(path("/999/chats.json"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(json!([campfire(FIRST_CAMPFIRE, BUCKET)]))
                .insert_header("Link", r#"<https://x/999/chats.json?page=2>; rel="next""#),
        )
        .mount(&server)
        .await;
    // One page allowed, a second named: the walk stops short and reports it, which is what
    // the listing cap looks like from `collect_all`.
    let mut config = Config::default().with_base_url(server.uri());
    config.max_pages = 1;
    let account = Client::builder(config)
        .access_token("test-token")
        .build()
        .unwrap()
        .for_account("999");
    let error = account
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap_err();
    assert!(
        matches!(
            RecordingSummaryError::of(&error),
            Some(RecordingSummaryError::CampfireDiscoveryIncomplete { .. })
        ),
        "got {error}"
    );
}

#[tokio::test]
async fn a_read_from_another_bucket_is_refused() {
    let server = MockServer::start().await;
    mount(
        &server,
        "GET",
        "/999/comments/1069479361",
        200,
        &comment(OTHER_BUCKET, "<div>hi</div>"),
    )
    .await;
    let error = account(&server)
        .recordings()
        .summarize(&RecordingRef::from_event(
            BUCKET,
            1_069_479_361,
            "comment.created",
        ))
        .await
        .unwrap_err();
    assert!(matches!(
        RecordingSummaryError::of(&error),
        Some(RecordingSummaryError::BucketMismatch {
            found_in: OTHER_BUCKET,
            ..
        })
    ));
    assert_eq!(error.code(), ErrorCode::NotFound);
}

#[tokio::test]
async fn a_pointer_without_ids_is_refused_before_any_request() {
    let server = MockServer::start().await;
    for reference in [
        RecordingRef::from_event(0, LINE, "comment.created"),
        RecordingRef::from_event(BUCKET, 0, "comment.created"),
        RecordingRef::from_event(-1, LINE, "comment.created"),
    ] {
        let error = account(&server)
            .recordings()
            .summarize(&reference)
            .await
            .unwrap_err();
        assert_eq!(error.code(), ErrorCode::Usage);
    }
    assert!(paths(&server).await.is_empty());
}

#[tokio::test]
async fn only_a_rich_text_line_reports_the_mentions_its_content_spells() {
    let markup = format!(r#"<div><bc-attachment sgid="{ANNIE_SGID}"></bc-attachment></div>"#);
    for (line_type, expected) in [
        ("Chat::Lines::RichText", vec![ANNIE]),
        ("Chat::Lines::Integration", vec![ANNIE]),
        // A plain-text or code line's content is text BC3 never read as markup, so a
        // literal bc-attachment in it mentions nobody.
        ("Chat::Lines::Text", vec![]),
        ("Chat::Lines::Code", vec![]),
    ] {
        let server = MockServer::start().await;
        mount(
            &server,
            "GET",
            &format!("/999/projects/{BUCKET}"),
            200,
            &project_with_chat(SECOND_CAMPFIRE),
        )
        .await;
        mount(
            &server,
            "GET",
            &line_path(SECOND_CAMPFIRE),
            200,
            &chat_line(BUCKET, SECOND_CAMPFIRE, line_type, &markup),
        )
        .await;
        let summary: RecordingSummary = account(&server)
            .recordings()
            .summarize(&RecordingRef::from_recording_type(BUCKET, LINE, line_type))
            .await
            .unwrap();
        assert_eq!(summary.mentioned_person_ids, expected, "{line_type}");
    }
}

#[tokio::test]
async fn hooks_see_the_constituent_reads_under_their_own_names() {
    let server = discovery_server(200).await;
    let log = Arc::new(HookLog::default());
    let account = Client::builder(Config::default().with_base_url(server.uri()))
        .access_token("test-token")
        .hooks(log.clone())
        .build()
        .unwrap()
        .for_account("999");
    account
        .recordings()
        .summarize(&chat_line_ref())
        .await
        .unwrap();
    let operations: Vec<String> = log
        .lines()
        .iter()
        .filter_map(|line| line.strip_prefix("op start ").map(str::to_string))
        .map(|line| line.split(' ').next().unwrap_or_default().to_string())
        .collect();
    assert_eq!(
        operations,
        vec![
            "GetProject",
            "ListCampfires",
            "GetCampfireLine",
            "GetCampfireLine"
        ],
        "SPEC section 18 rule 3: no synthetic composite operation name"
    );
}

#[tokio::test]
async fn create_with_mentions_reads_every_person_before_it_posts() {
    let server = MockServer::start().await;
    mount(
        &server,
        "GET",
        &format!("/999/people/{ANNIE}"),
        200,
        &person(ANNIE, Some(ANNIE_SGID)),
    )
    .await;
    mount(
        &server,
        "POST",
        "/999/recordings/1069479351/comments.json",
        201,
        &comment(BUCKET, "posted"),
    )
    .await;
    let account = account(&server);
    account
        .comments()
        .create_with_mentions(1_069_479_351, "<div>On it.</div>", &[ANNIE, ANNIE])
        .await
        .unwrap();
    let requests = server.received_requests().await.unwrap_or_default();
    assert_eq!(requests.len(), 2, "the repeated id is read once");
    let body: Value = serde_json::from_slice(&requests[1].body).unwrap();
    assert_eq!(
        body["content"],
        json!(format!(
            r#"<div><bc-attachment sgid="{ANNIE_SGID}"></bc-attachment> On it.</div>"#
        ))
    );
}

#[tokio::test]
async fn a_failed_person_read_posts_nothing() {
    let server = MockServer::start().await;
    mount(
        &server,
        "GET",
        &format!("/999/people/{ANNIE}"),
        403,
        &json!({ "error": "forbidden" }),
    )
    .await;
    let error = account(&server)
        .comments()
        .create_with_mentions(1_069_479_351, "<div>On it.</div>", &[ANNIE])
        .await
        .unwrap_err();
    assert_eq!(error.code(), ErrorCode::Forbidden);
    assert!(error.to_string().contains("resolving mention for person"));
    // Context is added to the message and nothing else: the read's own error travels
    // whole, so a caller still classifies it through the ordinary accessors.
    assert_eq!(error.http_status(), Some(403));
    assert_eq!(paths(&server).await.len(), 1, "nothing was posted");
}

#[tokio::test]
async fn an_empty_mention_list_posts_the_content_untouched_with_no_people_read() {
    let server = MockServer::start().await;
    mount(
        &server,
        "POST",
        "/999/recordings/1069479351/comments.json",
        201,
        &comment(BUCKET, "posted"),
    )
    .await;
    account(&server)
        .comments()
        .create_with_mentions(1_069_479_351, "<div>On it.</div>", &[])
        .await
        .unwrap();
    let requests = server.received_requests().await.unwrap_or_default();
    assert_eq!(requests.len(), 1);
    let body: Value = serde_json::from_slice(&requests[0].body).unwrap();
    assert_eq!(body["content"], json!("<div>On it.</div>"));
}

#[tokio::test]
async fn an_unusable_pointer_or_mention_id_is_refused_before_any_request() {
    let server = MockServer::start().await;
    let account = account(&server);
    assert_eq!(
        account
            .comments()
            .create_with_mentions(1, "", &[ANNIE])
            .await
            .unwrap_err()
            .code(),
        ErrorCode::Usage
    );
    let bad_id: Error = account
        .comments()
        .expand_mentions("hi", &[0])
        .await
        .unwrap_err();
    assert_eq!(bad_id.code(), ErrorCode::Usage);
    assert!(paths(&server).await.is_empty());
}

#[test]
fn the_documented_routing_sets_are_published() {
    let types = summarizable_recording_types();
    assert!(types.contains(&"Chat::Lines::*".to_string()));
    assert!(types.contains(&"Kanban::Card".to_string()));
    assert_eq!(
        summarizable_event_types(),
        vec![
            "card.*".to_string(),
            "chat.line.*".to_string(),
            "comment.*".to_string(),
            "message.*".to_string(),
            "todo.*".to_string(),
        ]
    );
}
