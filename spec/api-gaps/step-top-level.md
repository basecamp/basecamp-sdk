---
gap: step-top-level
status: absorbed-in-sdk
detected: 2026-05-01
sdk_demand: low
bc3_pr: 12323
smithy_refs:
  - "GetCardStep (spec/basecamp.smithy:4456)"
  - "CreateCardStep (spec/basecamp.smithy:4479)"
  - "UpdateCardStep (spec/basecamp.smithy:4511)"
  - "SetCardStepCompletion (spec/basecamp.smithy:4541)"
  - "RepositionCardStep (spec/basecamp.smithy:4569)"
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3b
  routes:
    - "GET /:account_id/card_tables/steps/:id.json (SDK-modeled; served but not listed in card_table_steps.md)"
    - "POST /:account_id/card_tables/cards/:card_id/steps.json"
    - "PUT /:account_id/card_tables/steps/:id.json"
    - "PUT /:account_id/card_tables/steps/:id/completions.json"
    - "POST /:account_id/card_tables/cards/:card_id/positions.json"
  controllers:
    - app/controllers/steps_controller.rb
  related_existing_api:
    - GetCardStep
    - CreateCardStep
    - UpdateCardStep
    - SetCardStepCompletion
    - RepositionCardStep
---

# Step (top-level paths)

## What's missing

Nothing — **absorbed**. The merged `doc/api/sections/card_table_steps.md` on
`master` (docs true-up, BC3 **#12323**) documents the top-level step routes —
`POST /card_tables/cards/:id/steps.json`, `PUT /card_tables/steps/:id.json`,
`PUT /card_tables/steps/:id/completions.json`,
`POST /card_tables/cards/:id/positions.json` (plus bucket-scoped
equivalents, e.g. `POST /buckets/:bucket_id/card_tables/cards/:id/steps.json` —
the doc lists these as cross-references to the same operations, and they are
**deliberately not modeled**: the SDK follows the card-tables service
convention of modeling the canonical flat `/card_tables/...` routes only;
the bucket-scoped forms are server-side aliases of the identical operations,
not additional API surface) — and the SDK already models all five top-level operations in
`spec/basecamp.smithy`: `GetCardStep` (:4456), `CreateCardStep` (:4479),
`UpdateCardStep` (:4511), `SetCardStepCompletion` (:4541),
`RepositionCardStep` (:4569).

The parameter check passes too: both `CreateCardStep` and `UpdateCardStep`
inputs carry `due_on: ISO8601Date` and `assignee_ids: PersonIdList`, matching
the merged doc. The doc's legacy `assignees` comma-separated-string param is
**deliberately unmodeled** in favor of the typed `assignee_ids` array.

## Why it matters

Historical: BC5 generalised Step beyond the Kanban-card context
(`Step::FormerlyKanbanStep` keeps `type: "Kanban::Step"` on the wire), and
this brief tracked whether the SDK's operations would line up with the
top-level paths BC3 documented. They do — no URL construction from
recording-id pairs is needed by SDK consumers.

## Suggested API shape

Shipped and modeled; see the Smithy refs above. The wire payload's `type`
field stays `"Kanban::Step"` per the `Step::FormerlyKanbanStep` override.

## Implementation notes for BC3

None — `doc/api/sections/card_table_steps.md` is the contract of record and
matches the SDK's modeled operations.

## SDK absorption plan when this lands

Done — no further absorption work:

- The five top-level operations exist with the documented paths and inputs.
- `CardStepsService` keeps its name; no parallel `StepsService` is needed.
- No new Smithy structures (the existing `CardStep` shape is reused).
- The legacy `assignees` comma-string param stays unmodeled by design; if BC3
  ever removes it from the doc, nothing changes here.
- Canary fixture: optional; the existing CardStep coverage already exercises
  the wire shape.
