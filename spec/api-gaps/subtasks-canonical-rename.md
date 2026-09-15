---
gap: subtasks-canonical-rename
status: addressed-in-bc3-pr-12659
detected: 2026-08-11
sdk_demand: medium
bc3_pr: 12659
bc3_refs:
  introduced_in: "step-to-subtask (BC3 #12544, merged 49eca3df973)"
  documented_in: "Add `Subtask` API (BC3 #12659, merged e8f0d765ba6)"
  documented_routes:
    - "GET /:account_id/recordings/:id/subtasks.json"
    - "POST /:account_id/recordings/:id/subtasks.json"
    - "GET /:account_id/subtasks/:id.json"
    - "PUT /:account_id/subtasks/:id.json"
    - "DELETE /:account_id/subtasks/:id.json"
    - "POST /:account_id/subtasks/:id/completion.json"
    - "DELETE /:account_id/subtasks/:id/completion.json"
    - "PUT /:account_id/subtasks/:id/position.json"
  sections:
    - doc/api/sections/subtasks.md
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

# Subtasks — bc3 documented the canonical surface, and it is wider than /steps

## The thing this brief was watching for has happened

BC3 **#12659** (`e8f0d765ba6`) adds `doc/api/sections/subtasks.md` and documents
eight routes under a top-level `/subtasks` and `/recordings/:id/subtasks`. The
"Why it matters" section below named this as the event that would turn a naming
detail into a contract change, so read the rest of the brief as the background
to it rather than as a live description of the gap.

Two things about what shipped, both of which move the SDK's position:

- **The documented spelling is neither of the two this brief tracked.** The
  canonical declarations were `/card_tables/subtasks/:id`; the documented
  aliases were `/card_tables/steps/:id`. What bc3 published is a *third*,
  card-table-free surface hung off recordings — `GET /recordings/3/subtasks`,
  `GET|PUT|DELETE /subtasks/:id`, `POST|DELETE /subtasks/:id/completion`,
  `PUT /subtasks/:id/position`. Re-pointing the five existing `@http` URIs, which
  is what the absorption plan below anticipated, is therefore *not* the change
  to make.
- **The surface generalised past card tables.** `subtasks.md` says a resource
  accepts subtasks if it exposes `subtasks_count` and `subtasks_url`, "today
  that means to-dos and cards". The SDK's five operations are card-step-shaped
  (`GetCardStep`, `CreateCardStep`, …) and reach steps *on cards only*. Subtasks
  on to-dos are reachable through the documented API and unreachable through
  this SDK.

The compatibility analysis below still holds exactly as written: the wire type
is still `Kanban::Step` — `subtasks.md` states the historical reason in its own
second paragraph — the `/steps` aliases are still permanent, and the five modeled
operations still work. Nothing is broken. What changed is that the SDK now models
a documented alias layer over a narrower slice of a documented resource, while
the general resource has a published contract it does not reach.

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

That was the state through the 2026-09-02 pin: the canonical routes the code
declared were invisible to `doc/api`, so `spec/bc3-routes.json` could not see
them and the SDK had nothing documented to model — `partial-coverage`, with the
gap on bc3's side.

#12659 closed the bc3 side and opened an SDK-side one. The routes are documented
now, `spec/bc3-routes.json` sees all eight, and what is missing is the modeling —
hence `addressed-in-bc3-pr-12659`, and hence the eight `bc3_routes_not_modeled`
entries in `spec/bc3-route-allowlist.yml` pointing here.

## Why it matters

Medium demand, raised from low when #12659 landed. Nothing is broken — the
documented `/steps` contract is fully served and the five modeled operations
are unaffected — so this is reach, not repair.

What raises it is the generalisation rather than the rename. Subtasks on to-dos
now have a published contract, and a caller holding a to-do with a non-zero
`subtasks_count` can read them with curl but not with this SDK. That is the
concrete gap; the spelling divergence recorded below is the history that
explains how the SDK ended up on the narrow side of it.

The paragraph this replaces predicted exactly this trigger — "a future bc3
change that documents `/subtasks` … turns this from a naming detail into a
contract change" — and it was right about the trigger while being wrong about
the shape, having assumed the documented spelling would be the canonical
`/card_tables/subtasks` one. Worth remembering when reading the confident parts
of any brief in this directory, including this one.

## Suggested API shape

A `Subtasks` service over the documented recording-scoped surface, sitting
alongside the five card-step operations rather than replacing them:

```
ListRecordingSubtasks:  GET    /{accountId}/recordings/{recordingId}/subtasks.json
CreateRecordingSubtask: POST   /{accountId}/recordings/{recordingId}/subtasks.json
GetSubtask:             GET    /{accountId}/subtasks/{subtaskId}.json
UpdateSubtask:          PUT    /{accountId}/subtasks/{subtaskId}.json
DeleteSubtask:          DELETE /{accountId}/subtasks/{subtaskId}.json
CompleteSubtask:        POST   /{accountId}/subtasks/{subtaskId}/completion.json
UncompleteSubtask:      DELETE /{accountId}/subtasks/{subtaskId}/completion.json
RepositionSubtask:      PUT    /{accountId}/subtasks/{subtaskId}/position.json
```

The payload is the shape `CardStep` already models — `subtasks.md`'s example
response carries `id`, `status`, `title`, `type: "Kanban::Step"`, `position`,
`url`, `app_url`, `bookmark_url`, `inherits_status` — so the existing structure
is the starting point, not a new one.

Deliberately additive. The `/steps` operations are documented, permanently
aliased and in use; retiring them is a separate decision under the no-alias
removal policy and should not ride along with adding reach to to-dos.

Two questions to settle against bc3 before modeling, neither answerable from
the docs alone: whether completion is genuinely `POST`/`DELETE` on a
`completion` singular (the card-step spelling is `PUT .../completions`, plural),
and whether `PUT /subtasks/:id/position` takes the same body as
`RepositionCardStep`'s `POST /card_tables/cards/:card_id/positions`.

## Implementation notes for BC3

- If the `/subtasks` spellings are ever meant to become the public contract,
  update `doc/api/sections/card_table_steps.md` (or a successor section) —
  until then the docs and the alias tests are self-consistent and nothing is
  required.
- The alias layer is pinned by `test/integration/route_aliases_test.rb`;
  removing any `/steps` form should fail there first.

## SDK absorption plan when this lands

It has landed upstream; what follows is the plan for the SDK PR that absorbs it.

- Add the eight operations above as a new `Subtasks` service group — new tag in
  `spec/overlays/tags.smithy`, new `TAG_TO_SERVICE` entry in all five service
  generators, accessor wiring in the TypeScript, Ruby and Python clients (Rust
  and Kotlin generate theirs).
- Do **not** re-point the five `CardStep` operations. That was this brief's
  earlier advice and the documented surface turned out not to match it.
- Reuse `CardStep`'s structure for the payload; the discriminator is still
  `"Kanban::Step"` and `json.steps` still names the embedded array, so the
  existing type work carries over. Confirm before assuming it: the compatibility
  contract is pinned by `test/integration/route_aliases_test.rb` for routes, not
  for this payload.
- Resolve the two spelling questions above against `config/routes.rb` and the
  bc3 API tests first — a wrong guess at `completion` vs `completions` is a live
  404, which is the failure mode `spec/bc3-route-allowlist.yml` exists to prevent.
- Cover the generalisation explicitly: a conformance case reading subtasks of a
  **to-do**, not just a card, since reaching to-dos is the point of absorbing this.
- Delete this file and its eight `bc3_routes_not_modeled` entries in that PR.
- [[step-top-level]] records how the `/steps` spellings were absorbed and stays
  the historical record for them; this brief owns the follow-through.
