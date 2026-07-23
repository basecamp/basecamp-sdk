---
gap: card-table-wormholes
status: absorbed-in-sdk
detected: 2026-07-23
sdk_demand: medium
smithy_refs:
  - "CreateWormhole (spec/basecamp.smithy:4673)"
  - "UpdateWormhole (spec/basecamp.smithy:4707)"
  - "DeleteWormhole (spec/basecamp.smithy:4741)"
  - "Wormhole (spec/basecamp.smithy:4816)"
bc3_refs:
  introduced_in: master
  routes:
    - "POST /:account_id/buckets/:bucket_id/card_tables/:card_table_id/wormholes.json"
    - "PUT /:account_id/buckets/:bucket_id/card_tables/wormholes/:id.json"
    - "DELETE /:account_id/buckets/:bucket_id/card_tables/wormholes/:id.json"
  controllers:
    - app/controllers/kanban/wormholes_controller.rb
  related_existing_api:
    - GetCardTable
    - MoveCard
    - CreateCardColumn
---

# Card-table wormholes (cross-project card moves)

> **Additive BC5 surface.** Unlike the contract-change entries
> ([dock-tool-create-contract](dock-tool-create-contract.md),
> [memories-emptied-regression](memories-emptied-regression.md)), this records
> new BC5 surface the SDKs could not express. The server contract is fully
> shipped and present in the pinned bc3 provenance (`338b7a11`, 2026-07-23; it
> also predates that pin, having landed by `ba105ba7`, 2026-07-22);
> `absorbed-in-sdk` means the SDK absorbed it without a provenance bump
> (basecamp-sdk#397).

## What's missing

A **wormhole** links a card table to a column on another card table and is the
only mechanism for moving a card to a *different project*. The pinned bc3
provenance already ships the full contract
(`app/controllers/kanban/wormholes_controller.rb`,
`app/views/api/kanban/wormholes/_wormhole.json.jbuilder`, `config/routes.rb`,
`doc/api/sections/card_table_wormholes.md`), but the SDKs could neither
*discover* wormholes nor *manage* them:

- **Discovery.** `GET /card_tables/{id}.json` emits a `wormholes[]` array (full
  recording shape plus `color`, `linked`, `destination_url`) that no SDK
  modeled — so `CardTables().Get` silently dropped it.
- **CRUD.** Three operations were unmodeled:
  - `POST /:account/buckets/:bucket/card_tables/:card_table/wormholes.json`
    → 201 (422 at the 4-per-board limit; 404 for an invalid, inaccessible,
    inactive, or same-board destination via a filtered `.find`).
  - `PUT /:account/buckets/:bucket/card_tables/wormholes/:id.json` → 200 (404).
  - `DELETE /:account/buckets/:bucket/card_tables/wormholes/:id.json`
    → 204 (403/404).

Routing a move *through* an existing wormhole already worked — a wormhole id is
a valid `column_id` for `MoveCard` — so the gap was purely discovery + CRUD,
not the move itself.

## Why it matters

Without `wormholes[]` decode and CRUD, cross-project card moves were
unreachable from the SDKs: a caller could not enumerate a board's wormholes to
find a teleport target, nor create/retarget/remove one. This blocked
basecamp-cli#342 / draft PR #559, which needs `cards wormholes --in <proj>`
(reads `wormholes[]`) and `cards move <id> --to-wormhole <id|dest-col-url>`.
The `destination_url` field is the *only* signal identifying a wormhole's
destination — the wormhole's own `url`/`app_url`/`parent` point at the source
board — so omitting it left the destination undiscoverable.

## Suggested API shape

None new — `master` already decided the shape. The SDK mirrors it: a single
shared `Wormhole` model (recording shape + `color`, `linked`,
`destination_url`) for both the `wormholes[]` decode member and the
create/update outputs, plus a dedicated `Wormholes` service exposing
`create(project, cardTable, destinationRecordingId)`,
`update(project, wormhole, destinationRecordingId)`, and
`delete(project, wormhole)`. `linked` is always emitted (true only while the
destination column, its board, and its bucket are all active);
`destination_url` is always present but nullable (null when unlinked).

## Implementation notes for BC3

None. The contract is shipped, documented
(`doc/api/sections/card_table_wormholes.md`), and inside the pinned provenance
(`338b7a11`). No upstream doc or route work remains.

## SDK absorption plan when this lands

Absorbed: basecamp-sdk#397 — three Smithy operations + shared `Wormhole`
structure + `wormholes: WormholeList` on `CardTable`, a dedicated `Wormholes`
service split across all six SDKs, the hand-written Go layer
(`go/pkg/basecamp/wormholes.go`), client wiring, and per-language tests
(CRUD happy + error paths, plus a linked/unlinked `wormholes[]` decode). No
`spec/api-provenance.json` SHA bump — the contract was already present at the
pinned `338b7a11` (and earlier). No further SDK change is needed.
