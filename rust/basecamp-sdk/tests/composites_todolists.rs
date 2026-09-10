//! SPEC §5 "Merge-Safe Write Surface (Todolists)", from
//! `conformance/tests/todolists_write.json`.

mod composites_support;

use basecamp_sdk::models::UpdateTodolistOrGroupRequestContent;
use basecamp_sdk::services::todolists::UpdateTodolistRequest;
use composites_support::run;

const FIXTURE: &str = "todolists_write";

#[tokio::test]
async fn update_merge_name_only_update_preserves_the_description() {
    run(
        FIXTURE,
        "update-merge: name-only update preserves the description",
        |account| async move {
            account
                .todolists()
                .update(
                    2,
                    &UpdateTodolistRequest {
                        name: Some("Renamed list".to_string()),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("updated");
}

#[tokio::test]
async fn update_group_the_same_composite_preserves_a_todolist_groups_description() {
    run(
        FIXTURE,
        "update-group: the same composite preserves a todolist group's description",
        |account| async move {
            account
                .todolists()
                .update(
                    2,
                    &UpdateTodolistRequest {
                        name: Some("Renamed group".to_string()),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("updated");
}

#[tokio::test]
async fn edit_clear_clearing_the_description_leaves_the_name_intact() {
    run(
        FIXTURE,
        "edit-clear: clearing the description leaves the name intact",
        |account| async move {
            account
                .todolists()
                .edit(2, |list| {
                    list.description = String::new();
                    Ok(())
                })
                .await
        },
    )
    .await
    .expect("edited");
}

#[tokio::test]
async fn replace_omission_clears_sparse_replace_sends_the_request_verbatim_with_no_get() {
    run(
        FIXTURE,
        "replace-omission-clears: sparse replace sends the request verbatim with no GET",
        |account| async move {
            account
                .todolists()
                .replace(
                    2,
                    &UpdateTodolistOrGroupRequestContent {
                        name: "The whole new list".to_string(),
                        description: None,
                    },
                )
                .await
        },
    )
    .await
    .expect("replaced");
}
