---
gap: subtasks-canonical-rename
status: partial-coverage
detected: 2026-08-11
sdk_demand: low
bc3_pr: 12544
bc3_refs:
  introduced_in: "step-to-subtask (BC3 #12544, merged 49eca3df973)"
  routes:
    - "GET /:account_id/card_tables/subtasks/:id.json (canonical; undocumented)"
    - "POST /:account_id/card_tables/cards/:card_id/subtasks.json (canonical; undocumented)"
    - "PUT /:account_id/card_tables/subtasks/:id.json (canonical; undocumented)"
    - "PUT /:account_id/card_tables/subtasks/:subtask_id/completions.json (canonical; undocumented)"
    - "POST /:account_id/card_tables/cards/:card_id/positions.json (canonical; path unchanged by the rename)"
  controllers:
    - app/controllers/subtasks_controller.rb (renamed from steps_controller.rb)
  related_existing_api:
    - GetCardStep
    - CreateCardStep
    - UpdateCardStep
    - SetCardStepCompletion
    - RepositionCardStep
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
