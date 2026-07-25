---
gap: activity-timeline
status: absorbed-in-sdk
detected: 2026-05-01
sdk_demand: high
bc3_pr: 11629
smithy_refs:
  - "TimelineEvent.kind/avatars_sample/data/attachments (spec/basecamp.smithy:7479)"
  - "TimelineEventData (spec/basecamp.smithy:7527)"
  - "TimelineAttachment (spec/basecamp.smithy:7543)"
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3d
  routes:
    - GET /:account_id/reports/progress.json
    - GET /:account_id/projects/:project_id/timeline.json
    - GET /:account_id/reports/users/progress/:person_id.json
  controllers:
    - app/controllers/timelines_controller.rb
    - app/controllers/users/timelines_controller.rb
  related_existing_api:
    - GetProgressReport
    - GetProjectTimeline
    - GetPersonProgress
---

# Activity Timeline (account, project, and person)

## What's missing

Additive fields only — the routes and base contract are documented and already
modeled. The merged `doc/api/sections/timeline.md` on `master` documents
exactly three routes:

- **Account** — `GET /reports/progress.json`: a paginated bare array of
  timeline events across all projects the authenticated user can access.
- **Project** — `GET /projects/:project_id/timeline.json`: the same event
  shape, pre-filtered to one project, also a bare array.
- **Person** — `GET /reports/users/progress/:person_id.json`: a JSON
  **object** `{person, events}` — the person plus a paginated `events` list of
  timeline events they created.

These routes **predate the BC5 API train**: they are a BC4-era contract,
documented since the #9981 docs repatriation, that BC5 kept. The train PR that
re-verified and regenerated `timeline.md` against live BC5 is BC3 **#11629**
(the doc-generation tooling PR), which is what `addressed-in-bc3-pr-11629`
records here.

Historical corrections retained from earlier drafts: there is no
`/activity.json` route and no `/buckets/:id/timeline` route; the BC5-new
`/activity/days/:date` and `/activity/dates` sub-routes were removed in the
timeline rewrite and were never modeled here.

## Why it matters

Activity feeds are a primary integration surface for dashboards, audit logs,
and "what's new since I last checked" tooling. The SDK already models all
three routes (operations `GetProgressReport`, `GetProjectTimeline`,
`GetPersonProgress` in `spec/basecamp.smithy`, including the person-route
`{person, events}` object wrapper), so consumers can call them today. This
absorption closes the remaining gap: the event payload's typed surface now
matches the merged doc.

## Suggested API shape

The remaining absorption is additive fields on the event shape, per the merged
`doc/api/sections/timeline.md` (the doc table is explicitly non-exhaustive —
"common values include" — and the live payload emits kinds the table omits,
e.g. `project_access_changed`, `dock_created`, `google_document_created`):

- `kind` — kept as an **open, non-exhaustive string** (documentation only, no
  closed enum). BC3 adds new kinds over time, so a closed enum would reject
  valid future values. Documented common values include `message_created`,
  `comment_created`, `todo_created`, `todo_completed`, `upload_created`,
  `document_created`, `google_document_created`, `schedule_entry_created`,
  `schedule_entry_rescheduled`, `question_created`, `question_answer_created`,
  `chat_transcript_rollup`, `kanban_card_created`, `kanban_card_completed`,
  `inbox_forward_created`, `client_correspondence_created`, `dock_created`, and
  `project_access_changed`.
- `data` — event-specific payload; present only for `schedule_entry_created` /
  `schedule_entry_rescheduled`, carrying `{all_day, starts_at, ends_at}`. Per
  the bc3 view (`starts_at_date_or_time`), `starts_at`/`ends_at` are
  **date-or-timestamp**: a full ISO 8601 timestamp for timed entries, or a bare
  date (`YYYY-MM-DD`) when `all_day` is true — so they are **not** plain
  timestamps. Modeled as `ISO8601Timestamp` (mirroring `ScheduleEntry`), with
  the Go enhancement mapping them to `types.FlexibleTime`; the other SDKs type
  them as plain strings.
- `avatars_sample` — array of avatar URLs (populated for chat rollups).
- `attachments` — **heterogeneous**, per the bc3 view
  (`api/timelines/events/_event.json.jbuilder`): an upload-kind recording
  contributes its full `uploads/upload` shape, while every other recording
  contributes a rich-text `attachments/attachment` (+ `blobs/blob`) partial.
  These variants share no required field set, so "reuse the existing attachment
  shape" is false. Modeled as an **optional-field superset struct**
  (`TimelineAttachment`) whose per-variant fields are all optional, so one
  element type decodes either variant (the cross-cutting untagged-polymorphism
  default; a union was not needed).
- Plus the documented envelope fields (`parent_recording_id`, `action`,
  `target`, `title`, `summary_excerpt`, `bucket`, `creator`, `url`,
  `app_url`), already modeled.

## Implementation notes for BC3

Shipped — nothing pending. The account and project routes serve from
`timelines_controller.rb` and the person route from
`users/timelines_controller.rb`; `doc/api/sections/timeline.md` is regenerated
against live BC5 by the doc tooling from #11629.

## SDK absorption plan when this lands

Absorbed (basecamp-sdk PR-2 of the post-#401 follow-up program).

- No new operations — `GetProgressReport`, `GetProjectTimeline`, and
  `GetPersonProgress` already existed with the correct paths and the
  person-route object wrapper.
- Extended `TimelineEvent` with the additive fields: documented `kind` as an
  open string (no closed enum), added `TimelineEventData` (`ISO8601Timestamp`
  starts_at/ends_at + Go `FlexibleTime` enhancement for all-day date-only
  values), `avatars_sample` (`StringList`), and the heterogeneous
  `attachments` array as the optional-field superset `TimelineAttachment`.
- **Runtime decode proof:** every SDK has a non-empty, per-variant response
  test that decodes BOTH attachment variants (a full Upload recording and a
  rich-text attachment/blob partial) in one array, plus the `data` all-day
  date-only payload and a non-empty `avatars_sample` — Go (`timeline_test.go`),
  TS, Ruby, Python, Kotlin, and Swift. Empty-array / generator-shape checks
  were treated as insufficient.
- Canary fixture: `GetProgressReport` exercises the account feed and validates
  statically (the live canary is dormant); the pairwise check is structural
  (the routes exist on both BC4 and BC5).
