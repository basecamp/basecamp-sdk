---
gap: everything-aggregates
status: no-json-contract
detected: 2026-05-01
sdk_demand: high
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3c
  routes:
    - "GET /:account_id/todos.json"
    - "GET /:account_id/todos/open.json"
    - "GET /:account_id/cards.json"
    - "GET /:account_id/cards/not_now.json"
    - "GET /:account_id/files.json"
    - "GET /:account_id/messages.json"
    - "GET /:account_id/comments.json"
    - "GET /:account_id/boosts.json"
    - "GET /:account_id/checkins.json"
    - "GET /:account_id/forwards.json"
    - "GET /:account_id/documents.json"
    - "(plus per-group filter sub-routes; final route list pending BC3 settlement)"
  controllers:
    - app/controllers/everything/todos_controller.rb
    - app/controllers/everything/cards_controller.rb
    - app/controllers/everything/files_controller.rb
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
There are 9 recording-type groups: todos, cards, files, messages, comments,
boosts, checkins, forwards, documents.

**Current BC3 Phase 3c scope: 22 API-eligible endpoints.** The earlier top
summary listed 30 endpoints across the 9 groups (bare top-level routes plus
filter sub-routes per group); the Phase 3c detail narrowed it to 22 by
excluding the bare top-level routes that stay HTML shells. Treat 22 as the
working number for absorption planning; if BC3 settles a different final
count before the absorption PR opens, this brief gets re-synced.

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

- 9 controllers under `app/controllers/everything/` already exist (web).
  Add a `respond_to :json` branch and corresponding jbuilder views.
- Consider whether to ship bare top-level routes (`/todos.json`, etc.) or
  scope to sub-routes only. Mirror that decision in `doc/api/`.
- Consistency: all 9 groups should use the same pagination + filter idiom.
  Inconsistency between groups creates per-endpoint absorption work in the SDK.

## SDK absorption plan when this lands

- New `EverythingService` with one op per endpoint BC3 ships.
- Each op routed at the flat top-level path (no `/everything/` URL prefix).
- Reuse existing recording shapes (`Todo`, `Card`, `Document`, `Message`, etc.).
- Canary fixture: cover at least one operation per group to catch shape drift.
  Pairwise check: BC4 absent → BC5 present is fine.
- Operation count in PR description must match what BC3 actually ships.
  Working number is 22 (Phase 3c API-eligible endpoints); brief author
  re-syncs route list before the absorption PR opens.
