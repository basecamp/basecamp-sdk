---
gap: everything-aggregates
status: absorbed-in-sdk
detected: 2026-05-01
sdk_demand: high
bc3_pr: 11627
smithy_refs:
  - "EverythingService flat family: GetEverythingMessages/Comments/Checkins/Forwards/Boosts/Files + GetEverythingOverdueTodos/OverdueCards (spec/basecamp.smithy)"
  - "EverythingFile superset (spec/basecamp.smithy)"
  - "EverythingService bucket-grouped family: GetEverythingOpen/Completed/Unassigned/NoDueDate Todos + Open/Completed/Unassigned/NoDueDate/NotNow Cards (spec/basecamp.smithy)"
  - "BucketTodosGroup / BucketCardsGroup (spec/basecamp.smithy)"
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
    - app/controllers/everything/files_controller.rb
  related_existing_api:
    - ListMyAssignments (similar contract — flat aggregate of one recording type)
---

# Everything aggregates (flat top-level recording listings)

## What's missing

SDK absorption only — the contract shipped. BC5's account-wide listings of
recordings by type are served by the `everything/*_controller.rb` namespace
under flat top-level paths (note: `/everything/...` is the **Rails controller
namespace**, not part of the URL). The contract merged to `master` via BC3
**#11627** as part of the BC5 API train (2026-07-18..21);
`doc/api/sections/everything.md` on `master` is the contract of record.

**Shipped scope: exactly 17 documented GET operations across 8 groups**
(re-derived from the merged doc's example markers):

- **todos ×5** — `/todos/{open,completed,overdue,unassigned,no_due_date}.json`
- **cards ×6** — `/cards/{open,completed,overdue,unassigned,no_due_date,not_now}.json`
- **flat roots ×6** — `/messages.json`, `/comments.json`, `/checkins.json`,
  `/forwards.json`, `/files.json`, `/boosts.json`

There is **no `/documents.json` root** — earlier drafts of this entry (working
from pre-merge #10947 heads) listed one, for an 18-op count. In the merged
contract, Basecamp documents surface through the `/files.json` feed instead,
alongside uploads and rich-text attachments.

Two standing exclusions from the merged doc:

- The bare `/todos.json` and `/cards.json` roots are **not JSON** — they are
  HTML shells in the web app. The filtered sub-routes are the JSON surface for
  those two groups.
- `/<resource>/recent.json` paths exist as internal web/Turbo-frame feeds and
  are explicitly **not** API contract: "those are internal: the root
  collection is the documented API contract. Don't depend on the `/recent`
  paths." The SDK must never model them.

## Why it matters

Without these, surfacing "all of one recording type across all projects" in a
custom integration requires walking projects and concatenating per-project
listings. The everything aggregates collapse that into a single paginated
request. This is a strong demand signal from the SDK side — the workaround is
painful and racy with project-membership changes.

## Suggested API shape

The merged `doc/api/sections/everything.md` documents two contract families:

- **Bucket-grouped lists** — the todo/card filter sub-routes
  (`/todos/{open,completed,unassigned,no_due_date}.json` and
  `/cards/{open,completed,unassigned,no_due_date,not_now}.json`) return a
  **paginated array of buckets** (Link-header pagination; observed live at 5
  buckets per page), each entry grouping the matching recordings — and their
  steps — under their parent project.
- **Flat recording lists** — `/todos/overdue.json` and `/cards/overdue.json`
  return a flat array of overdue recordings sorted oldest-first by due date;
  the 6 roots (`/messages.json`, `/comments.json`, `/checkins.json`,
  `/forwards.json`, `/files.json`, `/boosts.json`) return flat,
  recency-ordered (newest-first), paginated recording arrays, each item
  embedding its `bucket` for project context.

`GET /files.json` additionally takes `kind`
(`all` | `images` | `pdfs` | `documents` | `videos`) and repeatable
`people_ids[]` query filters, and mixes uploads, Basecamp documents, and
rich-text attachments (attachments wrapped in a recording envelope plus
`attachable_sgid` and blob metadata).

## Implementation notes for BC3

Shipped — nothing pending. 8 controllers under `app/controllers/everything/`
serve the 17 operations. The bare `/todos.json` and `/cards.json` roots stay
HTML shells, and `/<resource>/recent.json` stays internal web surface, per the
merged doc.

## SDK absorption plan when this lands

Absorbed in **two phases** across a stacked PR pair. All 17 routes are now
modeled, generated across the six SDKs, Go-wrapped, and decode-tested — the
entry is `absorbed-in-sdk`.

**PR-5 (flat family) — DONE.** A new `EverythingService` with the 8 flat-family
operations:

- Six recency-ordered, Link-paginated roots — `GetEverythingMessages`,
  `GetEverythingComments`, `GetEverythingCheckins`, `GetEverythingForwards`
  (element = the generic `Recording` projection the wire actually returns, which
  embeds `bucket`), `GetEverythingBoosts` (element = `Boost`, carrying its
  `booster` and nested `recording`), and `GetEverythingFiles`.
- Two unpaginated, oldest-due-date-first arrays — `GetEverythingOverdueTodos`
  (`Todo`) and `GetEverythingOverdueCards` (`Card`) — modeled as plain full
  arrays (single-member output, no pagination → bare array via the
  `smithy-bare-arrays` transform), tested as complete oldest-first with **no**
  Link-following.
- `/files.json` is **heterogeneous** (Upload + Document + attachment-envelope);
  modeled as the optional-field superset `EverythingFile` (the cross-cutting
  untagged-polymorphism default), with the `kind` and repeatable `people_ids[]`
  query filters. **Runtime decode proof:** every SDK (Go, TS, Ruby, Python,
  Kotlin, Swift) has a non-empty test decoding all three variants in one array.
- The generator auto-derives the `EverythingService` from the `Everything` tag;
  client wiring added where hand-written (Go accessor, TS `defineService` **plus
  package-root re-export**, Ruby `def everything`, Python sync+async properties;
  Kotlin/Swift auto-wired). `EverythingFile`/`Boost` added to the TS and Kotlin
  generator type registries so the paginated methods advertise `ListResult<T>`
  rather than a raw array alias; the generator now resolves `$ref` array aliases
  so the unpaginated overdue methods no longer mis-advertise `dict`/`Hash`.
- Each paginated flat op carries a `page` `@httpQuery` param, forwarded by the
  Go wrapper (a positive `page` fetches that page; `page 0` follows the Link
  header) — previously the Go `page` argument was silently ignored.
- `EverythingFile` optional timestamps/booleans are pointer-preserving (Go) so
  re-marshaling the superset omits absent-variant fields instead of fabricating
  a zero timestamp or dropping an explicit `false`.
- Go wrappers with multi-page Link-following (paginated roots) and plain
  full-array decode (overdue); the flat-aggregate mirror of `GetMyAssignments`.
- **Tests:** Go multi-page/overdue/files/page-forwarding tests, plus happy-path
  per-op tests for all 8 flat ops in TS/Ruby/Python and per-variant `/files.json`
  decode in all six SDKs.
- **Conformance/canary:** `paths.json` path-assertion entries + mock-runner
  dispatch (Go/TS/Kotlin) for all 8 flat ops; a live-canary entry per group in
  `live-my-surface.json` with matching `live-dispatch` cases (live canary
  dormant → validates statically).

**PR-5b (bucket-grouped family) — DONE.** The 9 todo/card filter sub-routes
(`/todos/{open,completed,unassigned,no_due_date}.json`,
`/cards/{open,completed,unassigned,no_due_date,not_now}.json`) return a
Link-paginated array of `{bucket, todos|cards}` (full `Todo`/`Card` recordings
with their embedded steps), modeled as `BucketTodosGroup` / `BucketCardsGroup`
(single-member output, Link pagination → bare array of groups). Go wrappers
(`OpenTodos`/`CompletedTodos`/`UnassignedTodos`/`NoDueDateTodos` and
`OpenCards`/`CompletedCards`/`UnassignedCards`/`NoDueDateCards`/`NotNowCards`)
with multi-page Link-following. Each op carries a `page` `@httpQuery` param
forwarded by the Go wrapper. The generator auto-derives these onto
`EverythingService` from the `Everything` tag (`BucketTodosGroup` /
`BucketCardsGroup` added to the TS + Kotlin type registries so the methods
advertise `ListResult<T>`). Coverage: Go multi-page/steps tests, happy-path
per-op tests in TS/Ruby/Python for all 9 ops, `paths.json` path-assertion
entries + mock-runner dispatch (Go/TS/Kotlin/Ruby/Python) for all 9, and a
live-canary entry per group with matching `live-dispatch` cases.

With both families landed, all 17 routes are modeled, generated, wrapped, and
covered — the entry is `absorbed-in-sdk`.

Exclusions honored: never model the `/<resource>/recent.json` aliases (internal
web feeds) or the bare `/todos.json` / `/cards.json` roots (HTML shells).
Pairwise check: BC4 absent → BC5 present is fine.
