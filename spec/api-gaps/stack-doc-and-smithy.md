---
gap: stack-doc-and-smithy
status: partial-coverage
detected: 2026-05-01
sdk_demand: medium
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3b
  routes:
    - GET /:account_id/stacks/:id.json
    - POST /:account_id/stacks.json
    - PUT /:account_id/stacks/:id.json
    - DELETE /:account_id/stacks/:id.json
    - GET /:account_id/stacks/:id/collectables.json
  controllers:
    - app/controllers/stacks_controller.rb
    - app/controllers/stacks/collectables_controller.rb
  related_existing_api:
    - "(stack jbuilder partial — already shipped; doc + Smithy still missing)"
---

# Stack — full CRUD + list collectables

## What's missing

Stacks (BC5's saved-collection abstraction) ship with a partial in
`app/views/api/stacks/_stack.json.jbuilder` already, but no documented JSON
contract or Smithy operations. The BC3 plan Phase 3b expanded this to **full
CRUD + list collectables** (revised from earlier "doc-only / add-remove
collectables" scope).

## Why it matters

Stacks are user-curated collections of recordings — first-class data the SDK
should be able to read and manipulate. Without CRUD, integrations can't
build "save to my stack" workflows or surface stack contents in dashboards.

## Suggested API shape

`GET /:account_id/stacks/:id.json`:
- `id`, `title`, `position`, `created_at`, `updated_at`
- `creator` (Person)
- `collectables_count`, `collectables_url`

`POST /:account_id/stacks.json`:
- Input: `title`, optional `position`
- Returns the created stack.

`PUT /:account_id/stacks/:id.json`:
- Input: subset of mutable fields (`title`, `position`).
- Returns the updated stack or 204.

`DELETE /:account_id/stacks/:id.json`:
- Returns 204.

`GET /:account_id/stacks/:id/collectables.json`:
- Pagination: Link header.
- Response: array of polymorphic Recording-shaped items.

## Implementation notes for BC3

- The stack partial already exists; show/index views reuse it.
- Add `create`/`update`/`destroy` actions with JSON branches.
- New `_collectable.json.jbuilder` (or reuse existing recording partial)
  for the collectables endpoint.
- `doc/api/sections/stacks.md` covers all five routes.

## SDK absorption plan when this lands

- New `StacksService` with `get(id)`, `create(input)`, `update(id, input)`,
  `delete(id)`, `listCollectables(id)`.
- New shapes: `Stack`, `CreateStackInput/Output`, `UpdateStackInput/Output`.
- `listCollectables` uses Recording polymorphic shape.
- Canary fixture: `getStack(id)` once a stable fixture exists; pairwise
  check is BC4-absent → BC5-present.
