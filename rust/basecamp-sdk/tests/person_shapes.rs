//! Person shapes the reference reads, and so writes back.
//!
//! Go decodes every embedded person through `generated.Person`, whose `Id` is
//! `types.FlexibleInt64`. Two shapes that are not id grammar follow from that:
//!
//! - an **absent** `id` is the zero value `0`, with no error — the flexible reader is only
//!   called for a key that is there — while an explicit `"id": null` reaches the reader and
//!   fails the read;
//! - a **`null` element** of a `[]Person` is the zero `Person`, contributing id `0`.
//!
//! A merge-safe write reads those lists back, so the reference sends `0` for either. Both
//! are generated-model optionality here (`rust/generator/src/emit/types.rs`), keyed on the
//! flexible id: a person type whose id is a plain `int64` in Go stays strict.
#![allow(clippy::unwrap_used, clippy::expect_used)]

mod support;

use basecamp_sdk::generated::types::{Person, Todo, UpcomingScheduleEntry, UpcomingSchedulePerson};
use basecamp_sdk::services::todos::UpdateTodoRequest;
use serde_json::{Value, json};
use wiremock::matchers::method;
use wiremock::{Mock, MockServer, ResponseTemplate};

/// A todo body the composites accept, with `assignees` and `completion_subscribers` replaced.
fn todo_with(assignees: &Value, completion_subscribers: &Value) -> Value {
    json!({
        "id": 456, "status": "active", "visible_to_clients": false,
        "created_at": "2024-01-15T10:00:00Z", "updated_at": "2024-01-15T10:00:00Z",
        "title": "Buy milk", "inherits_status": true, "type": "Todo",
        "url": "https://3.basecampapi.com/999/buckets/1/todos/456.json",
        "app_url": "https://3.basecamp.com/999/buckets/1/todos/456",
        "bookmark_url": "https://3.basecampapi.com/999/my/bookmarks/abc123.json",
        "content": "Buy milk", "description": "<p>From the store</p>",
        "completed": false, "comments_count": 0, "position": 1,
        "parent": {"id": 2, "title": "Todolist", "type": "Todolist",
                   "url": "https://3.basecampapi.com/999/buckets/1/todolists/2.json",
                   "app_url": "https://3.basecamp.com/999/buckets/1/todolists/2"},
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "creator": {"id": 1, "name": "Test User"},
        "assignees": assignees,
        "completion_subscribers": completion_subscribers,
        "description_attachments": []
    })
}

fn assignee_ids(body: &Value) -> Result<Vec<i64>, serde_json::Error> {
    let todo: Todo = serde_json::from_value(todo_with(body, &json!([])))?;
    Ok(todo
        .assignees
        .unwrap_or_default()
        .iter()
        .map(|p| p.id)
        .collect())
}

#[test]
fn an_absent_person_id_is_zero_and_a_null_one_fails_the_read() {
    let person: Person = serde_json::from_value(json!({"name": "A"})).unwrap();
    assert_eq!(person.id, 0);

    assert!(serde_json::from_value::<Person>(json!({"id": null, "name": "A"})).is_err());
    assert!(
        serde_json::from_value::<Person>(
            json!({"id": null, "name": "A", "personable_type": "User"})
        )
        .is_err()
    );
}

#[test]
fn a_null_element_in_a_person_list_is_the_zero_person() {
    assert_eq!(assignee_ids(&json!([null])).unwrap(), vec![0]);
    assert_eq!(
        assignee_ids(&json!([null, {"id": 7, "name": "A"}])).unwrap(),
        vec![0, 7]
    );
    assert_eq!(
        assignee_ids(&json!([{"id": 7, "name": "A"}, null])).unwrap(),
        vec![7, 0]
    );
    assert_eq!(
        assignee_ids(&json!([{"name": "A"}, {"id": "7", "name": "B"}])).unwrap(),
        vec![0, 7]
    );

    let zero: Todo = serde_json::from_value(todo_with(&json!([null]), &json!([]))).unwrap();
    assert_eq!(zero.assignees.unwrap()[0], Person::default());
}

#[test]
fn only_the_element_is_lenient() {
    // A null or absent list is still no list, not a list of one zero person.
    assert_eq!(assignee_ids(&json!(null)).unwrap(), Vec::<i64>::new());
    let todo: Todo = serde_json::from_value({
        let mut body = todo_with(&json!([]), &json!([]));
        body.as_object_mut().unwrap().remove("assignees");
        body
    })
    .unwrap();
    assert_eq!(todo.assignees, None);

    // An element that is not an object, an explicit null id and a float id still fail.
    assert!(assignee_ids(&json!([5])).is_err());
    assert!(assignee_ids(&json!([{"id": null, "name": "A"}])).is_err());
    assert!(assignee_ids(&json!([{"id": 1024.0, "name": "A"}])).is_err());
    assert!(assignee_ids(&json!({"id": 7})).is_err());
}

/// `UpcomingSchedulePerson.ID` is a plain `int64` in Go, not the flexible reader: the rule
/// above is the flexible id's, and does not reach it.
#[test]
fn a_person_type_without_the_flexible_id_stays_strict() {
    assert!(
        serde_json::from_value::<UpcomingSchedulePerson>(json!({"name": "A", "avatar_url": ""}))
            .is_err()
    );

    let mut entry = serde_json::to_value(UpcomingScheduleEntry::default()).unwrap();
    entry["participants"] = json!([{"id": 7, "name": "A", "avatar_url": ""}]);
    assert!(serde_json::from_value::<UpcomingScheduleEntry>(entry.clone()).is_ok());
    entry["participants"] = json!([null]);
    assert!(serde_json::from_value::<UpcomingScheduleEntry>(entry).is_err());
}

/// Through the merge-safe composite: the reference sends `0` for both shapes.
#[tokio::test]
async fn a_merge_safe_update_writes_back_zero_for_an_absent_id_and_a_null_element() {
    let server = MockServer::start().await;
    let read = todo_with(
        &json!([{"name": "A"}, null, {"id": 7, "name": "B"}]),
        &json!([null]),
    );
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(200).set_body_json(&read))
        .mount(&server)
        .await;
    Mock::given(method("PUT"))
        .respond_with(ResponseTemplate::new(200).set_body_json(todo_with(&json!([]), &json!([]))))
        .mount(&server)
        .await;

    support::account(&server)
        .todos()
        .update(
            456,
            &UpdateTodoRequest {
                content: Some("x".into()),
                ..Default::default()
            },
        )
        .await
        .unwrap();

    let requests = server.received_requests().await.unwrap();
    let put = requests
        .iter()
        .find(|r| r.method.as_str() == "PUT")
        .unwrap();
    let body: Value = serde_json::from_slice(&put.body).unwrap();
    assert_eq!(body["assignee_ids"], json!([0, 0, 7]));
    assert_eq!(body["completion_subscriber_ids"], json!([0]));
}
