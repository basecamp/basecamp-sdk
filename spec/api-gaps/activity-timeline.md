---
gap: activity-timeline
status: no-json-contract
detected: 2026-05-01
sdk_demand: high
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3d
  routes:
    - GET /:account_id/activity.json
    - GET /:account_id/projects/:project_id/timeline.json
  controllers:
    - app/controllers/activity_controller.rb
    - app/controllers/timelines_controller.rb
  related_existing_api: []
---

# Activity Timeline (global + per-project)

## What's missing

BC5 introduces a global activity feed (`/activity.json`) and per-project
timeline (`/projects/:project_id/timeline.json`) backed by an Activity
envelope. The BC3 plan Phase 3d ships both routes plus the envelope shape.

Note: there is **no `/buckets/:id/timeline` route** — earlier drafts of this
brief used the wrong path. Per-project routes are keyed by `project_id`, not
`bucket_id`.

## Why it matters

Activity feeds are a primary integration surface for dashboards, audit logs,
and "what's new since I last checked" tooling. Without a JSON API, consumers
have to poll individual recording endpoints and reconstruct ordering — slow,
incomplete, and incompatible with the in-app activity stream.

## Suggested API shape

`GET /:account_id/activity.json`:
- Pagination: Link header, `X-Total-Count`.
- Response: array of `Activity` envelope objects:
  - `id` (long), `created_at`, `updated_at`
  - `kind` (string — what happened)
  - `recording` (polymorphic: Todo, Card, Message, etc.)
  - `creator` (Person)
  - `bucket` (TodoBucket-shaped, when scoped)

`GET /:account_id/projects/:project_id/timeline.json`:
- Same envelope, pre-filtered to the project.
- Pagination: same as global.

## Implementation notes for BC3

- Activity envelope likely already exists in some form (used by web UI). BC3
  Phase 3d work makes it a documented, JSON-serialisable contract.
- Per-project timeline action lives on `timelines_controller.rb`, not on
  `projects_controller`.
- `doc/api/sections/activity.md` covers both the global and per-project
  routes plus the envelope shape.

## SDK absorption plan when this lands

- New `ActivityService` with `getActivity()` (global) and
  `getProjectTimeline(projectId)` (per-project).
- New shape: `Activity` envelope.
- The recording field is polymorphic — model as a union of existing recording
  shapes if Smithy supports it, otherwise use a discriminated structure with
  a `type` field.
- Canary fixture: include `getActivity()` once schema is stable. Pairwise
  check is BC4-absent → BC5-present.
