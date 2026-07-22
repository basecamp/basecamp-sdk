---
gap: everything-aggregates
status: addressed-in-bc3-pr-11627
detected: 2026-05-01
sdk_demand: high
bc3_pr: 11627
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

- New `EverythingService` with **17** operations matching the merged contract
  (one per endpoint in the frontmatter route list). Re-verify the operation
  list against `doc/api/sections/everything.md` at absorption time.
- Two response families: a bucket-grouped shape for the todo/card filter
  sub-routes (bucket + grouped recordings + steps) and flat recording arrays
  for the overdue lists and the 6 roots. Reuse existing recording shapes
  (`Todo`, `Card`, `Message`, etc.) for the elements.
- Do **not** model the `/<resource>/recent.json` aliases — internal web feeds,
  not API contract. Do not model bare `/todos.json` / `/cards.json` (HTML
  shells).
- Model the `/files.json` `kind` and `people_ids[]` query filters.
- Canary fixture: cover at least one operation per group (8 groups) to catch
  shape drift. Pairwise check: BC4 absent → BC5 present is fine.
