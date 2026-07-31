---
gap: everything-todo-card-filters
status: addressed-in-bc3-pr-12442
detected: 2026-07-30
sdk_demand: medium
bc3_pr: 12442
bc3_refs:
  introduced_in: master
  routes:
    - "GET /:account_id/todos/open.json"
    - "GET /:account_id/todos/completed.json"
    - "GET /:account_id/todos/overdue.json"
    - "GET /:account_id/todos/unassigned.json"
    - "GET /:account_id/todos/no_due_date.json"
    - "GET /:account_id/cards/open.json"
    - "GET /:account_id/cards/completed.json"
    - "GET /:account_id/cards/overdue.json"
    - "GET /:account_id/cards/unassigned.json"
    - "GET /:account_id/cards/no_due_date.json"
    - "GET /:account_id/cards/not_now.json"
  controllers:
    - app/controllers/concerns/everything/recording_filters.rb
    - app/controllers/concerns/everything/todos/recordings.rb
    - app/controllers/concerns/everything/cards/recordings.rb
  related_existing_api:
    - GetEverythingOpenTodos (and the rest of the everything to-do/card family the filters attach to)
---

# Assignee and due filters on the everything to-do/card API

## What's missing

BC3 **#12442** (merged 2026-07-28, `b238a0743`) added two optional query
filters to **every** everything to-do and card endpoint — the bucket-grouped
family and the flat overdue lists alike. Per
`doc/api/sections/everything.md` ("Filtering to-dos and cards"):

- `assignee_ids[]` — one or more person IDs (repeatable). Returns only tasks
  assigned to at least one of the requested people. Matching considers the
  task's own assignees; assignees on nested steps are not considered. If none
  of the requested IDs resolve to people the caller knows, the result is
  empty.
- `due` — one of `with`, `without`, or `overdue`. Returns only tasks that
  have a due date, have none, or are past due. Unrecognized values are
  ignored.

The SDK models the 11 affected operations (see
[everything-aggregates.md](everything-aggregates.md)) but none of them carry
these query parameters yet.

## Why it matters

Without the filters, narrowing "everything" listings by assignee or due
status means fetching the full paginated feed and filtering client-side —
wasteful on large accounts and racy against concurrent changes. The filters
also compose (`?assignee_ids[]=N&due=overdue`), which client-side filtering
can only approximate at full-feed cost.

## Suggested API shape

Add to each of the 11 everything to-do/card operations' inputs:

- `assignee_ids` — repeatable `@httpQuery("assignee_ids[]")` list of person
  IDs, following the established bracketed-array pattern
  (`people_ids[]` on `GetEverythingFiles`).
- `due` — optional `@httpQuery("due")` string (`with` | `without` |
  `overdue`).

## Implementation notes for BC3

Shipped — nothing pending. The shared
`Everything::RecordingFilters` concern applies both filters in the
`filtered_recordings` chain for the todos and cards recordings concerns;
`doc/api/sections/everything.md` on `master` is the contract of record.

## SDK absorption plan when this lands

Add the two query params to the 11 operation inputs in
`spec/basecamp.smithy`, regenerate all six SDKs, thread the new options
through the Go wrappers (`EverythingService` bucket-grouped and overdue
methods), and cover with per-SDK request-construction tests (repeatable
`assignee_ids[]` serialization mirrors the `people_ids[]` precedent on
`GetEverythingFiles`). Conformance: extend the existing everything paths
fixtures with a query-parameter assertion per family.
