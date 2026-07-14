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
    - "GET /:account_id/documents.json"
    - "GET /:account_id/messages.json"
    - "GET /:account_id/comments.json"
    - "GET /:account_id/checkins.json"
    - "GET /:account_id/forwards.json"
    - "GET /:account_id/files.json"
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
    - app/controllers/everything/files_controller.rb
  related_existing_api:
    - ListMyAssignments (similar contract — flat aggregate of one recording type)
---

# Everything aggregates (flat top-level recording listings)

## What's missing

BC5 introduces account-wide flat listings of recordings by type, served by the
`everything/*_controller.rb` namespace under flat top-level paths (note:
`/everything/...` is the **Rails controller namespace**, not part of the URL).
There are 9 groups: todos, cards, messages, comments, boosts, checkins,
forwards, documents, and files.

**Current BC3 scope: 18 documented GET operations across 9 groups**, per
`doc/api/sections/everything.md` in BC3 PR #10947 (open/unmerged, head
`589b1970`, verified 2026-07-13): todos ×5 and cards ×6 via filtered
sub-routes (`open`, `completed`, `overdue`, `unassigned`, `no_due_date`, plus
`not_now` for cards), and 7 root-path feeds — `/documents.json`,
`/messages.json`, `/comments.json`, `/checkins.json`, `/forwards.json`,
`/files.json`, `/boosts.json`.

Superseded interim history: the launch reconciliation
(`BRIEF-bc5-reconciliation-scope-cuts.md`, `five+api` @ `716e710ee5`) briefly
scoped this to a 17-operation surface with the files group excluded, and
framed the feed groups as `recent`-suffixed sub-routes. #10947 has since
**restored the files group** (`GET /files.json`, with `kind` and
`people_ids[]` filters)
and documented the feed groups at their **root paths**. Only the bare
`/todos.json` and `/cards.json` roots remain HTML shells — use the filtered
sub-routes for those two groups.

The `/<resource>/recent.json` paths still exist in the web app as internal
Turbo-frame feeds, but they are **not** API surface. Per
`doc/api/sections/everything.md`: "those are internal: the root collection is
the documented API contract. Don't depend on the `/recent` paths." The SDK
must not model them.

## Why it matters

Today, surfacing "all of one recording type across all projects" in a custom
integration requires walking projects and concatenating per-project listings.
The everything aggregates collapse that into a single paginated request.
This is a strong demand signal from the SDK side — the workaround is painful
and racy with project-membership changes.

## Suggested API shape

`doc/api/sections/everything.md` (#10947, head `589b1970`) documents two
contract families:

- **Bucket-grouped lists** — the todos/cards filter sub-routes (`/todos/open.json`,
  `/cards/not_now.json`, …) return a paginated array of buckets, each grouping
  the matching recordings (and their steps) under their parent project.
- **Flat recording lists** — `/todos/overdue.json`, `/cards/overdue.json`, and
  the root feeds (`/documents.json`, `/messages.json`, `/comments.json`,
  `/checkins.json`, `/forwards.json`, `/files.json`, `/boosts.json`) return a
  flat array of recording objects, each embedding its `bucket` for project
  context. Feeds are newest-first and paginated; overdue lists sort
  oldest-first by due date.

`GET /files.json` additionally takes `kind` (`all` | `images` | `pdfs` |
`documents` | `videos`) and repeatable `people_ids[]` query filters, and mixes
uploads with rich-text attachments (attachments wrapped in a recording
envelope plus `attachable_sgid` and blob metadata).

Root feeds are documented at their root paths; todos and cards are reachable
only via the filtered sub-routes (their bare roots stay HTML shells).

## Implementation notes for BC3

- 9 controllers under `app/controllers/everything/` (the files group is back
  in scope — see What's missing).
- Feed groups serve JSON at their root paths; the bare `/todos.json` and
  `/cards.json` roots stay HTML shells with the filtered sub-routes as the
  JSON surface. `/<resource>/recent.json` stays an internal web Turbo-frame
  feed — keep it out of `doc/api/`.
- Consistency: all 9 groups should use the same pagination + filter idiom.
  Inconsistency between groups creates per-endpoint absorption work in the SDK.

## SDK absorption plan when this lands

- New `EverythingService` with **18** operations matching the documented
  contract (one per endpoint in the frontmatter route list), opening once BC3
  PR #10947 merges. Re-derive the operation list from
  `doc/api/sections/everything.md` at the merge head at absorption time.
- Do **not** model the `/<resource>/recent.json` aliases unless explicitly
  approved — they are internal web feeds, not API contract.
- Each op routed at the flat top-level path (no `/everything/` URL prefix).
- Reuse existing recording shapes (`Todo`, `Card`, `Document`, `Message`, etc.).
- Canary fixture: cover at least one operation per group (9 groups) to catch
  shape drift. Pairwise check: BC4 absent → BC5 present is fine.
- Operation count in the PR description must match what #10947 actually ships
  (18 as of head `589b1970`, 2026-07-13); re-sync the route list if BC3
  settles a different final count before the absorption PR opens.
