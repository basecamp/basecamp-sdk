---
gap: stack-doc-and-smithy
status: confirmed-not-api-resource
detected: 2026-05-01
sdk_demand: medium
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3b
  bc3_plan_section: "BC3 reconciliation handoff §1 (BRIEF-bc5-reconciliation-scope-cuts.md, five+api @ 716e710ee5); withdrawn from API scope at launch"
  routes:
    - "(none — Stacks render web-only on four and master; no respond_to :json, no doc/api section)"
  controllers:
    - app/controllers/stacks_controller.rb
  related_existing_api: []
---

# Stack — withdrawn from API scope at launch (web-only)

> **Classification: not an API resource.** Retained as the durable record of
> why no `StacksService` is modeled. Registry rule: briefs trump the allowlist,
> so this stays a brief rather than moving to `allowlist.yml`.

## What's missing

Nothing for the SDK to absorb. At the BC5 launch reconciliation, Stacks were
confirmed **web-only on both BC4 (`four`) and BC5 (`master`)**:

- `StacksController` renders with `layout false` and has **no `respond_to` /
  `format.json` branch** — there is no bearer-token JSON contract.
- There is **no `doc/api/sections/stacks.md`**.
- `Stacks::CollectablesController` **does not exist on `master`**.
- The `app/views/api/stacks/_stack.json.jbuilder` partial renders only when
  **nested inside other responses**, never as a standalone Stack endpoint.

Earlier drafts of this brief assumed BC3 Phase 3b would ship full Stack CRUD +
list-collectables JSON. The reconciliation handoff
(`BRIEF-bc5-reconciliation-scope-cuts.md`, `five+api` @ `716e710ee5`) withdrew
that: there is no public JSON Stack contract on either branch. The BC5 API
train (2026-07-18..21) confirmed the classification — no Stack section shipped
with it.

**Post-launch update (2026-07-21):** the product has renamed Stacks to
**Folders**, and an API for them is being scoped server-side. The wire `type`
remains `Stack`. Neither the rename nor the scoping changes this entry today —
there is still no JSON contract to model.

## Why it matters

It matters as a *negative* result. The SDK explicitly does **not** model a
`StacksService`, and the canary does not expect Stack endpoints on either
backend. Recording that here prevents a future detector run or contributor from
re-filing Stacks as an additive gap. When the server-side Folders API scoping
produces a JSON contract, file a fresh additive brief then (watch the wire
`type`, which stays `Stack` despite the product rename).

## Suggested API shape

None. Stacks have no public JSON request/response shape on `four` or `master`.
The nested `_stack` partial's fields are an implementation detail of whatever
parent response embeds it, not a standalone contract, so there is no shape to
propose.

## Implementation notes for BC3

No SDK-facing action yet. The Folders API being scoped server-side is net-new
API work: `respond_to :json` branches, standalone jbuilder views, a `doc/api/`
section, and whatever collectables surface it defines. None of that exists
today on either branch.

## SDK absorption plan when this lands

No absorption. **No `StacksService`** — none of `Get` / `Create` / `Update` /
`Delete` / `ListCollectables` are modeled, because none exist on the BC3 wire.
This brief is the classification record; `status: confirmed-not-api-resource`
reflects the launch decision. Should a JSON contract appear later, supersede
this entry with a fresh additive brief.
