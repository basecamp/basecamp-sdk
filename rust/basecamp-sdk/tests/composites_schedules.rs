//! SPEC §5 "Merge-Safe Write Surface (Schedule Entries)", from
//! `conformance/tests/schedule_entries_write.json`. The three `create-*` cases are plain
//! `create_entry` wire cases that share the file; they are reproduced here as well.

mod composites_support;

use basecamp_sdk::FlexibleTime;
use basecamp_sdk::models::{CreateScheduleEntryRequestContent, ReplaceScheduleEntryRequestContent};
use basecamp_sdk::services::schedules::UpdateScheduleEntryRequest;
use composites_support::run;

const FIXTURE: &str = "schedule_entries_write";
const ENTRY: i64 = 1_069_479_523;
const SCHEDULE: i64 = 1_069_479_520;

#[tokio::test]
async fn replace_omission_clears_an_unaddressed_participant_list_join_link_and_highlight_stay_off_the_wire()
 {
    run(
        FIXTURE,
        "replace-omission-clears: an unaddressed participant list, join link and highlight stay off the wire",
        |account| async move {
            account
                .schedules()
                .replace_entry(
                    ENTRY,
                    &ReplaceScheduleEntryRequestContent {
                        summary: Some("Team Meeting".to_string()),
                        starts_at: "2026-06-05T06:00:00Z".to_string(),
                        ends_at: "2026-06-05T08:30:00Z".to_string(),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("replaced");
}

#[tokio::test]
async fn replace_clears_carve_outs_explicit_empty_values_reach_the_wire() {
    run(
        FIXTURE,
        "replace-clears-carve-outs: explicit empty values reach the wire",
        |account| async move {
            account
                .schedules()
                .replace_entry(
                    ENTRY,
                    &ReplaceScheduleEntryRequestContent {
                        summary: Some("Team Meeting".to_string()),
                        starts_at: "2026-06-05T06:00:00Z".to_string(),
                        ends_at: "2026-06-05T08:30:00Z".to_string(),
                        participant_ids: Some(Vec::new()),
                        url: Some(String::new()),
                        highlighted: Some(false),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("replaced");
}

#[tokio::test]
async fn replace_single_request_the_raw_path_never_reads_before_writing() {
    run(
        FIXTURE,
        "replace-single-request: the raw path never reads before writing",
        |account| async move {
            account
                .schedules()
                .replace_entry(
                    ENTRY,
                    &ReplaceScheduleEntryRequestContent {
                        summary: Some("Offsite".to_string()),
                        starts_at: "2026-07-01T09:00:00Z".to_string(),
                        ends_at: "2026-07-01T17:00:00Z".to_string(),
                        description: Some("<div>Whole day.</div>".to_string()),
                        all_day: Some(true),
                        participant_ids: Some(vec![1_049_715_914]),
                        url: Some("https://meet.example.com/offsite".to_string()),
                        highlighted: Some(true),
                        notify: None,
                    },
                )
                .await
        },
    )
    .await
    .expect("replaced");
}

#[tokio::test]
async fn update_merge_a_summary_only_update_preserves_the_times_description_and_all_day_flag() {
    run(
        FIXTURE,
        "update-merge: a summary-only update preserves the times, description and all-day flag",
        |account| async move {
            account
                .schedules()
                .update_entry(
                    ENTRY,
                    &UpdateScheduleEntryRequest {
                        summary: Some("Team Meeting & Kickoff".to_string()),
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
async fn update_addresses_carve_outs_a_caller_set_join_link_and_highlight_reach_the_wire() {
    run(
        FIXTURE,
        "update-addresses-carve-outs: a caller-set join link and highlight reach the wire",
        |account| async move {
            account
                .schedules()
                .update_entry(
                    ENTRY,
                    &UpdateScheduleEntryRequest {
                        url: Some("https://meet.example.com/new-room".to_string()),
                        highlighted: Some(true),
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
async fn update_clears_carve_outs_an_explicitly_empty_join_link_empty_participant_list_and_false_highlight_are_sent()
 {
    run(
        FIXTURE,
        "update-clears-carve-outs: an explicitly empty join link, empty participant list and false highlight are sent",
        |account| async move {
            account
                .schedules()
                .update_entry(
                    ENTRY,
                    &UpdateScheduleEntryRequest {
                        url: Some(String::new()),
                        highlighted: Some(false),
                        participant_ids: Some(Vec::new()),
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
async fn edit_clear_clearing_the_description_keeps_every_other_field() {
    run(
        FIXTURE,
        "edit-clear: clearing the description keeps every other field",
        |account| async move {
            account
                .schedules()
                .edit_entry(ENTRY, |entry| {
                    entry.description = String::new();
                    Ok(())
                })
                .await
        },
    )
    .await
    .expect("edited");
}

#[tokio::test]
async fn edit_untouched_carve_outs_a_block_that_never_assigns_them_leaves_them_off_the_wire() {
    run(
        FIXTURE,
        "edit-untouched-carve-outs: a block that never assigns them leaves them off the wire",
        |account| async move {
            account
                .schedules()
                .edit_entry(ENTRY, |entry| {
                    assert_eq!(entry.url(), Some("https://meet.example.com/team"));
                    assert_eq!(entry.highlighted(), Some(true));
                    assert_eq!(
                        entry.participant_ids(),
                        Some(&[1_049_715_914, 1_049_715_915][..])
                    );
                    entry.summary = "Team Sync".to_string();
                    Ok(())
                })
                .await
        },
    )
    .await
    .expect("edited");
}

#[tokio::test]
async fn edit_touched_carve_outs_assigning_the_value_the_read_already_returned_still_sends_it() {
    run(
        FIXTURE,
        "edit-touched-carve-outs: assigning the value the read already returned still sends it",
        |account| async move {
            account
                .schedules()
                .edit_entry(ENTRY, |entry| {
                    let url = entry.url().unwrap_or_default().to_string();
                    entry.set_url(url);
                    let highlighted = entry.highlighted().unwrap_or_default();
                    entry.set_highlighted(highlighted);
                    Ok(())
                })
                .await
        },
    )
    .await
    .expect("edited");
}

#[tokio::test]
async fn create_join_link_url_highlighted_and_status_reach_the_wire_on_create() {
    run(
        FIXTURE,
        "create-join-link: url, highlighted and status reach the wire on create",
        |account| async move {
            account
                .schedules()
                .create_entry(
                    SCHEDULE,
                    &CreateScheduleEntryRequestContent {
                        summary: "Kickoff call".to_string(),
                        starts_at: "2026-06-05T06:00:00Z".to_string(),
                        ends_at: "2026-06-05T08:30:00Z".to_string(),
                        url: Some("https://zoom.us/j/999".to_string()),
                        highlighted: Some(true),
                        status: Some("drafted".to_string()),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("created");
}

#[tokio::test]
async fn create_omits_unset_an_unset_join_link_highlight_and_status_stay_off_the_wire() {
    run(
        FIXTURE,
        "create-omits-unset: an unset join link, highlight and status stay off the wire",
        |account| async move {
            account
                .schedules()
                .create_entry(
                    SCHEDULE,
                    &CreateScheduleEntryRequestContent {
                        summary: "Kickoff call".to_string(),
                        starts_at: "2026-06-05T06:00:00Z".to_string(),
                        ends_at: "2026-06-05T08:30:00Z".to_string(),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("created");
}

#[tokio::test]
async fn create_all_day_bare_date_a_bare_date_reaches_the_wire_on_create_matching_replace() {
    run(
        FIXTURE,
        "create-all-day-bare-date: a bare date reaches the wire on create, matching replace",
        |account| async move {
            account
                .schedules()
                .create_entry(
                    SCHEDULE,
                    &CreateScheduleEntryRequestContent {
                        summary: "Offsite".to_string(),
                        starts_at: "2026-06-01".to_string(),
                        ends_at: "2026-06-02".to_string(),
                        all_day: Some(true),
                        ..Default::default()
                    },
                )
                .await
        },
    )
    .await
    .expect("created");
}

/// Not a fixture case: the bounds are round-tripped verbatim, so an all-day entry's bare
/// dates survive a summary-only edit unparsed and unrendered.
#[tokio::test]
async fn edit_round_trips_bare_date_bounds_verbatim() {
    let entry = run(
        FIXTURE,
        "edit-untouched-carve-outs: a block that never assigns them leaves them off the wire",
        |account| async move {
            account
                .schedules()
                .edit_entry(ENTRY, |entry| {
                    entry.starts_at = FlexibleTime::from("2026-06-01");
                    entry.ends_at = FlexibleTime::from("2026-06-02");
                    entry.all_day = true;
                    entry.summary = "Team Sync".to_string();
                    Ok(())
                })
                .await
        },
    )
    .await
    .expect("edited");
    assert_eq!(entry.summary, "Team Sync");
}
