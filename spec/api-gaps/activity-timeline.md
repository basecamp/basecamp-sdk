---
gap: activity-timeline
status: addressed-in-bc3-pr-11629
detected: 2026-05-01
sdk_demand: high
bc3_pr: 11629
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
three routes (`GetProgressReport`, `GetProjectTimeline`, `GetPersonProgress`
at `spec/basecamp.smithy:7084`, `:7105`, `:7130`, including the person-route
`{person, events}` object wrapper), so consumers can call them today — but the
event payload's typed surface lags the merged doc, leaving fields to be
consumed untyped.

## Suggested API shape

The remaining absorption is additive fields on the event shape, per the merged
`doc/api/sections/timeline.md`:

- `kind` — a 15-value vocabulary: `message_created`, `comment_created`,
  `todo_created`, `todo_completed`, `upload_created`, `document_created`,
  `schedule_entry_created`, `schedule_entry_rescheduled`, `question_created`,
  `question_answer_created`, `chat_transcript_rollup`, `kanban_card_created`,
  `kanban_card_completed`, `inbox_forward_created`,
  `client_correspondence_created`.
- `data` — event-specific payload; for `schedule_entry_created` /
  `schedule_entry_rescheduled` it carries `{all_day, starts_at, ends_at}`.
- `avatars_sample` — array of avatar URLs (used by chat rollups to show
  participants).
- `attachments` — array of attached files, if any.
- Plus the documented envelope fields (`parent_recording_id`, `action`,
  `target`, `title`, `summary_excerpt`, `bucket`, `creator`, `url`,
  `app_url`).

## Implementation notes for BC3

Shipped — nothing pending. The account and project routes serve from
`timelines_controller.rb` and the person route from
`users/timelines_controller.rb`; `doc/api/sections/timeline.md` is regenerated
against live BC5 by the doc tooling from #11629.

## SDK absorption plan when this lands

- No new operations — `GetProgressReport`, `GetProjectTimeline`, and
  `GetPersonProgress` already exist with the correct paths and the
  person-route object wrapper.
- Extend the timeline event shape with the additive fields above: the `kind`
  vocabulary (model as an open string enum — BC3 says "common values
  include", so treat it as non-exhaustive), the `data` struct for
  schedule-entry events, `avatars_sample`, and `attachments`.
- Canary fixture: `GetProgressReport` exercises the account feed; pairwise
  check is structural (the routes exist on both BC4 and BC5).
