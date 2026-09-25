---
gap: subtasks-canonical-rename
status: absorbed-in-sdk
detected: 2026-08-11
sdk_demand: medium
bc3_pr: 12659
smithy_refs:
  - ListSubtasks
  - GetSubtask
  - CreateSubtask
  - UpdateSubtask
  - CompleteSubtask
  - UncompleteSubtask
  - RepositionSubtask
  - DeleteSubtask
  - "Todo.subtasks_count / subtasks_completed_count / subtasks_url"
  - "Card.subtasks_count / subtasks_completed_count / subtasks_url"
  - "Recording.subtasks_count / subtasks_completed_count / subtasks_url"
bc3_refs:
  introduced_in: "step-to-subtask (BC3 #12544, merged 49eca3df973); documented and given its canonical flat routes by Add Subtask API (BC3 #12659, e8f0d765ba6)"
  routes:
    - "GET /:account_id/recordings/:recording_id/subtasks.json (paginated)"
    - "GET /:account_id/subtasks/:id.json"
    - "POST /:account_id/recordings/:recording_id/subtasks.json"
    - "PUT /:account_id/subtasks/:id.json"
    - "POST /:account_id/subtasks/:id/completion.json"
    - "DELETE /:account_id/subtasks/:id/completion.json"
    - "PUT /:account_id/subtasks/:id/position.json"
    - "DELETE /:account_id/subtasks/:id.json"
  controllers:
    - app/controllers/subtasks_controller.rb
    - app/controllers/subtasks/completions_controller.rb
    - app/controllers/subtasks/positions_controller.rb
  related_existing_api:
    - GetCardStep
    - CreateCardStep
    - UpdateCardStep
    - SetCardStepCompletion
    - RepositionCardStep
    - CardStep
---

# Subtasks — the canonical routes moved out from under the documented /steps spellings

## What's missing

BC3 **#12544** (`4547876f10b`, merged `49eca3df973`) renamed Step to Subtask
throughout — controllers, views, models, and routes. The canonical route
declarations are now `resources :subtasks`: the flat card-table forms read
`/card_tables/subtasks/:id` and `/card_tables/cards/:card_id/subtasks`, and the
bucket-scoped forms follow. **None of the canonical `/subtasks` spellings is
documented** — `doc/api/sections/card_table_steps.md` is untouched by the
rename and still documents only the `/steps` forms.

Nothing breaks for the SDK, deliberately:

- **Every `/steps` route form is re-declared as a permanent alias.** The
  rename's own commit message says the migration plan keeps them ("Clients
  hold those paths and doc/api documents them, so they don't get removed"),
  and `test/integration/route_aliases_test.rb` (+92 lines in the same commit)
  pins each spelling — the flat `/card_tables/steps` family explicitly
  annotated "which doc/api documents. Kept indefinitely."
- **The wire payload is byte-identical.** `_step.json.jbuilder` moved to
  `_subtask.json.jbuilder` as a 100% rename; cards and todos still emit the
  array under `json.steps`; the type discriminator stays `"Kanban::Step"` via
  `Subtask::FormerlyKanbanStep` (previously `Step::FormerlyKanbanStep`); and
  the emitted `url` / `completion_url` still render the `/steps` spellings,
  pinned to the documented literals by the same test.
- **All five SDK-modelled operations keep working.** `GetCardStep`,
  `CreateCardStep`, `UpdateCardStep`, `SetCardStepCompletion` now resolve via
  the permanent aliases; `RepositionCardStep`'s
  `POST /card_tables/cards/:card_id/positions` never had "steps" in its path,
  so it is still the canonical declaration (only its controller was renamed).

BC3 **#12639** (`e6408768f52`) makes that compatibility contract explicit in
application code. `Kanban::Step` is frozen as the API type for subtasks that
originated as card steps, and webhook events render `kind` through
`Event#api_kind` so the same public discriminator survives independently of
the renamed model. The route declarations now point at `Kanban::StepsController`
and `Kanban::Steps::CompletionsController` without changing any documented
path. `CardStep.type` and `Event.kind` already model strings, so the SDK contract
remains current.

What is missing is on the bc3 side: the canonical routes the code now
declares are invisible to `doc/api`, so `spec/bc3-routes.json` cannot see
them and the SDK has nothing documented to model. Documented spellings and
canonical declarations have diverged — that combination is
`partial-coverage`.

## Why it matters

Low demand: the documented contract is fully served and the SDK's operations
are unaffected. The brief exists so the divergence is on record before it
compounds. bc3's docs now describe an alias layer, not the canonical routes;
a future bc3 change that documents `/subtasks`, emits `/subtasks` URLs in
payloads, renames the `json.steps` key, or changes the `"Kanban::Step"`
discriminator turns this from a naming detail into a contract change. Whoever
triages that range should start from this entry rather than rediscover the
alias topology.

## Suggested API shape

None yet — the SDK deliberately models the documented `/steps` spellings and
should keep doing so while they are the documented contract. If bc3
re-documents the surface under `/subtasks`, the operations' `@http` URIs move
(or gain modelled siblings) at that point, not before.

## Implementation notes for BC3

- If the `/subtasks` spellings are ever meant to become the public contract,
  update `doc/api/sections/card_table_steps.md` (or a successor section) —
  until then the docs and the alias tests are self-consistent and nothing is
  required.
- The alias layer is pinned by `test/integration/route_aliases_test.rb`;
  removing any `/steps` form should fail there first.

## SDK absorption plan when this lands

Nothing to absorb today. If bc3 documents the canonical `/subtasks` routes:

- Re-point the five operations' `@http` URIs (or add documented siblings) —
  wire shapes are unchanged, so no structure work is expected.
- Watch the payload keys: absorption is only mechanical while `json.steps`
  and `"Kanban::Step"` survive on the wire; if either moves with the docs,
  the `CardStep` structure and its consumers need a real pass.
- [[step-top-level]] records how the `/steps` spellings were absorbed and
  stays the historical record for them; this brief owns the canonical-rename
  follow-through.

## As of BC3 #12659 (`e8f0d765ba6`): documented, and absorbed as `SubtasksService`

bc3 documented the canonical surface in a new `doc/api/sections/subtasks.md`
and reshaped it after comments: the parent advertises `subtasks_count`,
`subtasks_completed_count` and `subtasks_url`, the index paginates the same
way, and every route is flat and speaks of subtasks — `GET`/`POST
/recordings/:id/subtasks.json`, `GET`/`PUT`/`DELETE /subtasks/:id.json`,
`POST`/`DELETE /subtasks/:id/completion.json` and `PUT
/subtasks/:id/position.json`. Reposition and completion take a to-do's shape
(a 1-based `position`; no `source_id`, no `completion: on/off`), and a
subtask's emitted `url` and `completion_url` now render the `/subtasks`
spellings. The embedded `steps` array on a to-do or card is capped at 100;
`subtasks_count` is the real total. `type` stays `"Kanban::Step"` permanently.

The SDK absorbed it as the `Subtasks` service (the eight operations in
`smithy_refs`), reusing the `CardStep` structure — the wire shape did not
change, only the routes and the parent's accounting did — and added the three
accounting members to `Todo`, `Card` and the generic `Recording` projection.
The `CardSteps` operations stay modelled exactly as before: bc3 keeps the
card-scoped `/steps` spellings served indefinitely through legacy controllers
of their own, and `card_table_steps.md` now points at the subtasks section as
the one to use for new integrations. That doc also corrected its
`position` parameter from "Zero indexed" to 1-based; the server always
counted from 1, so `RepositionCardStep`'s member documentation and the Go
wrapper's lower bound moved with it.

Two fixtures under `spec/fixtures/subtasks/` are the documented examples and
are validated as `CardStep` by `make check-fixture-coverage`; `todos/get.json`
and `cards/get.json` gained the three accounting keys.
