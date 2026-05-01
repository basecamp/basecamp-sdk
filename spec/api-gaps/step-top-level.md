---
gap: step-top-level
status: partial-coverage
detected: 2026-05-01
sdk_demand: low
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3b
  routes:
    - "GET /:account_id/buckets/:bucket_id/cards/:card_id/steps/:id.json (existing — already in SDK)"
    - "Top-level Step paths (final path/depth pending BC3 doc decision)"
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

BC5 generalises Step beyond the Kanban-card context (`Step::FormerlyKanbanStep`
keeps `type: "Kanban::Step"` on the wire — see SDK plan §Out of scope). The
jbuilder partial for Step is already shipped via the cards routes. The BC3
parity plan Phase 3b adds a doc-only entry exposing top-level Step routes
(unscoped from cards).

The wire shape itself is unchanged — the SDK already models `CardStep`. What's
missing is documentation + Smithy operations under the new top-level paths.

## Why it matters

If BC3 documents top-level Step routes, the SDK needs corresponding Smithy ops
so SDK consumers can use them without manually constructing URLs from
recording-id pairs. Forward compat is fine: the existing `CardStepsService`
keeps working under its current paths.

## Suggested API shape

Same as existing `CardStep` shape (already modelled at `spec/basecamp.smithy`
line 4712). The new operations are merely routed differently — most likely:

- `GET /:account_id/steps/:id.json` (single Step regardless of recording parent)
- `PUT /:account_id/steps/:id/completions.json` (toggle completion)

The wire payload's `type` field stays `"Kanban::Step"` per BC3 plan's
`Step::FormerlyKanbanStep` override.

## Implementation notes for BC3

- Choose and document the canonical top-level path.
- Reuse the existing `_step.json.jbuilder` partial.
- Update `doc/api/sections/cards.md` (or add `doc/api/sections/steps.md`) to
  describe both forms.

## SDK absorption plan when this lands

- Either extend `CardStepsService` with a new `getStep(id)` op routed at
  `/:account_id/steps/:id.json`, OR add a parallel `StepsService` with the
  same shape — coordinate naming with the BC3 doc choice.
- Do not rename `CardStepsService` — the existing service stays.
- No new Smithy structures (existing `CardStep` is reused).
- Canary fixture: optional; the existing CardStep coverage already exercises
  the wire shape.
