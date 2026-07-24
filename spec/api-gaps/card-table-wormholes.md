---
gap: card-table-wormholes
status: addressed-in-bc3-pr-12144
detected: 2026-07-23
sdk_demand: low
bc3_pr: 12144
bc3_refs:
  introduced_in: master
  routes:
    - "POST /:account_id/buckets/:bucket_id/card_tables/:card_table_id/wormholes.json (create — destination_recording_id, 201; 422 if the table already holds 4)"
    - "PUT /:account_id/buckets/:bucket_id/card_tables/wormholes/:id.json (update destination — destination_recording_id, 200)"
    - "DELETE /:account_id/buckets/:bucket_id/card_tables/wormholes/:id.json (delete — 204)"
  controllers:
    - app/controllers/kanban/wormholes_controller.rb
  related_existing_api:
    - MoveCard
    - GetCardTable
---

# Card table wormholes — CRUD documented, unmodeled in the SDK

## What's missing

A **wormhole** sends cards dropped into it straight to a column on **another
card table** (including one in a different project) — the mechanism behind a
cross-project card move. The wormhole CRUD contract itself shipped in BC3
**#12144** ("Wormholes", `e41d2e33da`, merged 2026-07-09), which added both
`doc/api/sections/card_table_wormholes.md` and
`app/controllers/kanban/wormholes_controller.rb`. That predates this PR's pin
range; what surfaced the SDK gap in-range is BC3 **#12385** (`90631fbf`,
2026-07-23), which spelled out the cross-project move recipe in
`doc/api/sections/card_table_cards.md` and leans on the #12144 CRUD surface:

- **Create** — `POST /buckets/:bucket_id/card_tables/:card_table_id/wormholes.json`,
  required `destination_recording_id` (a column on another card table the caller
  can access) → `201 Created` with the wormhole JSON, or `422` if the table
  already holds the max of four wormholes.
- **Update** — `PUT /buckets/:bucket_id/card_tables/wormholes/:id.json`, required
  `destination_recording_id` → `200 OK` (the wormhole's title follows its
  destination).
- **Delete** — `DELETE /buckets/:bucket_id/card_tables/wormholes/:id.json` → `204`.
- Wormholes surface in a card table's `wormholes` array (see Get a card table),
  titled "Project › Board › Column". Only **linked** wormholes (`"linked": true`)
  with a reachable destination accept a card; moving onto an unlinked or
  unreachable one returns `404`.

The SDK models `MoveCard` (whose `column_id` may be a wormhole id) but has **no
wormhole shapes and no create/update/delete operations** — there is no `wormhole`
anywhere in `spec/basecamp.smithy`, and the card-table structure carries no
`wormholes` field. So a consumer can *move* a card through an existing wormhole
but cannot create, retarget, or delete one, and can't read a table's wormholes
to discover a target id — the cross-project move recipe #12385 documents is not
expressible end-to-end through the typed SDK.

This entry **registers** the documented contract; it does **not** absorb it.

## Why it matters

Cross-project card moves are the concrete use case: without wormhole CRUD, an
integration can't set up (or tear down) the teleport it then drives with
`MoveCard`. `sdk_demand` is low — wormholes are a niche Kanban feature, capped at
four per table, and the move itself already works once a linked wormhole exists.

## Suggested API shape

Model a `Wormhole` structure (matching the `wormholes` array element on the card
table) plus three operations: `CreateCardTableWormhole`
(`POST …/card_tables/:id/wormholes.json`, body `{destination_recording_id}` →
201), `UpdateCardTableWormhole` (`PUT …/card_tables/wormholes/:id.json` → 200),
and `DeleteCardTableWormhole` (`DELETE …/card_tables/wormholes/:id.json` → 204).
The card-table response structure **must** carry the `wormholes` field — it is
the **only** way to discover an existing wormhole's id (create returns the new
one, but there is no list-wormholes endpoint), so update/delete/reuse are
unreachable without it. Note the `linked`/reachable precondition and the
max-four (`422`) rule in the docs.

## Implementation notes for BC3

Done: the CRUD surface shipped in #12144 (`e41d2e33da`) and lives in
`doc/api/sections/card_table_wormholes.md`, handled by
`app/controllers/kanban/wormholes_controller.rb`. #12385 (`90631fbf`) later
tightened the cross-project move recipe in `card_table_cards.md` (linked-only
precondition → 404, `position` ignored on a teleport, queue-then-204 timing).
Nothing further is required of BC3 for the register step.

## SDK absorption plan when this lands

Deferred. When absorbed, add the `Wormhole` structure and the three CRUD
operations to Smithy, add the required card-table `wormholes` field (the sole id
discovery surface), regenerate, and flip this entry to `absorbed-in-sdk` with the
Smithy refs. Until then it stays `addressed-in-bc3-pr-12144` as a register-only
record.
