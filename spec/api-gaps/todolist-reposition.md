---
gap: todolist-reposition
status: absorbed-in-sdk
detected: 2026-07-22
sdk_demand: medium
# #9575 landed the route (pre-BC5); #11674 shipped the doc tail in
# doc/api/sections/todolists.md, pulled in by the 338b7a11 repin.
bc3_pr: "9575, 11674"
smithy_refs:
  - "RepositionTodolist (spec/basecamp.smithy:1120)"
bc3_refs:
  introduced_in: master
  bc3_plan_phase: pre-BC5
  routes:
    - "PUT /:account_id/todosets/todolists/:id/position.json"
  controllers:
    - app/controllers/todosets/todolists/positions_controller.rb
  related_existing_api:
    - GetTodolistOrGroup
    - RepositionTodolistGroup
---

# To-do list reposition ships without documented JSON API coverage

> **Long-standing, undocumented route.** The dedicated to-do-list position
> route landed in BC3 `a61b90d00f` via PR #9575 (pre-BC5) and was live as of the
> `ba105ba7` (2026-07-22) pin when this entry was written (the pin has since
> advanced to `338b7a11`), exercised by
> `test/api/todosets/todolists/positions_controller_api_test.rb`. It was
> undocumented in `doc/api/` when this entry was written; **BC3 #11674**
> ("Document repositioning to-do lists in the API") has since shipped that doc
> tail (`doc/api/sections/todolists.md`), pulled into the `338b7a11` repin.
> `absorbed-in-sdk` here means the SDK added `RepositionTodolist` against a
> contract already live upstream (basecamp-cli#484 follow-up); the SDK
> absorption itself was no-repin, no new upstream delivery.

## What's missing

The SDK had no way to reorder a whole to-do list within its to-do set. The
generated client exposed `RepositionTodo`, `RepositionTodolistGroup`,
`RepositionCardStep`, and `RepositionTool`, but nothing bound to the dedicated
to-do-list position route.

The route itself is live and covered upstream:
`PUT /:account_id/todosets/todolists/:id/position.json` →
`Todosets::Todolists::PositionsController#update` (bc3 `config/routes.rb`,
`app/controllers/todosets/todolists/positions_controller.rb`), asserted to
return `:no_content` (204) by
`test/api/todosets/todolists/positions_controller_api_test.rb` at pin
`ba105ba7`. The remaining gap was documentation — and BC3 **#11674** has since
documented it at `doc/api/sections/todolists.md` ("Reposition a to-do list":
`PUT /todosets/todolists/:id/position.json` → 204, `position` param), pulled in
by the `338b7a11` repin. The `bc3-api` mirror sync (bc-api#415) is the tail.

## Why it matters

Integrations that build or reorder project structure could not move a to-do
list relative to its siblings. The generic recording reposition
(`PUT /recordings/:id/position.json`, the `RepositionTool` binding) is **not a
correct substitute**: `Recordings::PositionsController#update` runs a bare
`reposition_to` with only dock translation and returns 200. It performs none
of the to-do-list position math — no `position_offset` for loose to-dos, no
hidden-completed-list translation — so a caller sending a simple visible index
through the generic route mis-positions the list whenever loose to-dos or
hidden completed lists exist.

## Suggested API shape

`PUT /:account_id/todosets/todolists/:id/position.json`, body
`{ position }` where `position` is the 1-based index among the to-do lists the
caller can see. The server applies the loose-to-do `position_offset` and
hidden-completed-list translation, shifts siblings to make room, and returns
**204 No Content**. This matches the dedicated controller already live
upstream.

## Implementation notes for BC3

Server-side behavior needs no change — the route, controller, and API test all
exist at the current pin. The only remaining gap was documentation, now closed:
BC3 **#11674** added a "Reposition a to-do list" section to the authoritative
`doc/api/sections/todolists.md` (`PUT /todosets/todolists/:id/position.json` —
the 1-based visible-index contract and the 204 response). The `bc3-api` public
mirror sync (bc-api#415) is the remaining tail.

## SDK absorption plan when this lands

Absorbed: `RepositionTodolist` tagged into the Todolists service, so the
generated `reposition` method lands on `TodolistsService` in every SDK (no
bucket ID). Go adds a hand-written `TodolistsService.Reposition` wrapper as the
user-facing entry point for basecamp-cli#484. Shipped with tests (Go, TS, Ruby,
Python) and conformance coverage of the declared 204. The only remaining tail
is the upstream doc addition described above.
