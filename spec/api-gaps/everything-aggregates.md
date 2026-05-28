---
gap: everything-aggregates
status: no-json-contract
detected: 2026-05-01
sdk_demand: high
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3c
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
    - "GET /:account_id/documents/recent.json"
    - "GET /:account_id/messages/recent.json"
    - "GET /:account_id/comments/recent.json"
    - "GET /:account_id/checkins/recent.json"
    - "GET /:account_id/forwards/recent.json"
    - "GET /:account_id/boosts.json"
  controllers:
    - app/controllers/everything/todos_controller.rb
    - app/controllers/everything/cards_controller.rb
    - app/controllers/everything/messages_controller.rb
    - app/controllers/everything/comments_controller.rb
    - app/controllers/everything/boosts_controller.rb
    - app/controllers/everything/checkins_controller.rb
    - app/controllers/everything/forwards_controller.rb
    - app/controllers/everything/documents_controller.rb
  related_existing_api:
    - ListMyAssignments (similar contract — flat aggregate of one recording type)
---

# Everything aggregates (flat top-level recording listings)

## What's missing

BC5 introduces account-wide flat listings of recordings by type, served by the
`everything/*_controller.rb` namespace under flat top-level paths (note:
`/everything/...` is the **Rails controller namespace**, not part of the URL).
There are 8 active groups after the launch reconciliation: todos, cards,
messages, comments, boosts, checkins, forwards, documents (the files group was
dropped — see below).

**Current BC3 scope: 17 API-eligible endpoints** (BC3 PR #10947, open/unmerged).
The launch reconciliation (`BRIEF-bc5-reconciliation-scope-cuts.md`, `five+api`
@ `716e710ee5`) dropped the **files group**: `master` consolidated file-type
routes into `GET /files/recent.json?kind=` and unions unrenderable attachment
recordings (no API recordable-partial), so files are out of #10947's scope. Net
22 → 17. Bare collection routes (`/todos.json`, `/cards.json`, …) are HTML
shells and intentionally do **not** return JSON; the API surface is the named
subroutes in the frontmatter plus standalone `/boosts.json`. Files-by-kind
(`/files/recent?kind=`) is a possible *future* BC3 deliverable — file a fresh
brief then; do not model it speculatively now.

## Why it matters

Today, surfacing "all of one recording type across all projects" in a custom
integration requires walking projects and concatenating per-project listings.
The everything aggregates collapse that into a single paginated request.
This is a strong demand signal from the SDK side — the workaround is painful
and racy with project-membership changes.

## Suggested API shape

Each endpoint mirrors the `ListMyAssignments`-shaped contract: a paginated
list of recordings of a single type, scoped to the current user's visibility.

Example: `GET /:account_id/todos/open.json`:
- Pagination: Link header (RFC5988), `X-Total-Count`, `maxPageSize: 50`.
- Response: array of Todo (per existing `_todo.json.jbuilder`).

Filter sub-routes follow the existing convention used by `MyAssignments` —
e.g. `/cards/not_now.json` is the per-status filter variant of `/cards.json`.

## Implementation notes for BC3

- 8 controllers under `app/controllers/everything/` already exist (web; the
  files group is out of scope — see What's missing). Add a `respond_to :json`
  branch and corresponding jbuilder views.
- Bare top-level routes (`/todos.json`, etc.) stay HTML shells; the JSON
  surface is the named subroutes only. Mirror that in `doc/api/`.
- Consistency: all 8 groups should use the same pagination + filter idiom.
  Inconsistency between groups creates per-endpoint absorption work in the SDK.

## SDK absorption plan when this lands

- New `EverythingService` with **17** operations (one per endpoint in the
  frontmatter route list), opening once BC3 PR #10947 merges.
- Each op routed at the flat top-level path (no `/everything/` URL prefix).
- Reuse existing recording shapes (`Todo`, `Card`, `Document`, `Message`, etc.).
- Canary fixture: cover at least one operation per group to catch shape drift.
  Pairwise check: BC4 absent → BC5 present is fine.
- Operation count in the PR description must match what #10947 actually ships
  (17 after the files-group drop); re-sync the route list if BC3 settles a
  different final count before the absorption PR opens.
