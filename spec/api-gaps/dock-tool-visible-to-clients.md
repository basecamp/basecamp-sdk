---
gap: dock-tool-visible-to-clients
status: addressed-in-bc3-pr-12386
detected: 2026-07-23
sdk_demand: low
bc3_pr: 12386
bc3_refs:
  introduced_in: master
  routes:
    - "POST /:account_id/buckets/:bucket_id/dock/tools.json (existing CreateTool — visible_to_clients additive, honored only by self-visibility tool types)"
  controllers:
    - app/controllers/docks/tools_controller.rb
  related_existing_api:
    - CreateTool
---

# Create-time `visible_to_clients` on dock tool creation

## What's missing

The dock-tool create endpoint (`POST /buckets/:bucket_id/dock/tools.json`,
modeled as `CreateTool`) gained an optional top-level `visible_to_clients`
boolean at create time in BC3 **#12386** ("Honor create-time visible_to_clients
on dock tool creates", merged `bee714c74`, 2026-07-23). This is the dock-tool
sibling of the six content creates absorbed in
[[visible-to-clients-on-creates]]; it is split into its own entry because the
honoring rule is narrower and `CreateToolInput` is **not** modeled with the
field yet.

The contract is narrower than the content creates:

- The flag **only takes effect for tool types that manage their own
  visibility** — `Chat::Transcript` and `Kanban::Board`, which otherwise start
  hidden from clients. Pass `true` to create one already client-visible.
- **All other tool types ignore it** and inherit the project's default.
- It applies **only when a new tool is created**; re-enabling an existing tool
  keeps its current visibility.
- It is a **top-level** boolean, a sibling of `tool_type`/`title` (documented in
  `doc/api/sections/tools.md`). `Docks::ToolsController` includes
  `Recording::VisibleToClientsParam` and gates it on
  `tool.changes_client_visibility?`.

The SDK's `CreateTool` request type does not yet expose `visible_to_clients`, so
consumers can't set a Chat/Kanban tool's client visibility at create time.

This entry **registers** the shipped contract; it does **not** absorb it. It is
kept separate from the six-content-create absorption on purpose: `CreateTool` is
a distinct dock-tool operation with a narrower (two-tool-type) honoring rule, so
folding it in would both widen scope and blur the semantics.

## Why it matters

Without it, creating a client-visible Chat transcript or Kanban board is a
two-step create-then-toggle, with a window where the tool is briefly hidden (or
visible) under the wrong setting. `sdk_demand` is low: it affects only two tool
types, and the create-then-toggle path (via the existing client-visibility
endpoints) works today.

## Suggested API shape

Additive optional `visible_to_clients: Boolean` (tri-state, same pointer/nullable
modeling as the content creates) on the existing `CreateToolInput`. Response
shape unchanged. When modeled, document the two-tool-type honoring rule so
callers don't expect it to apply to, e.g., a `Vault` or `Message::Board` tool.

## Implementation notes for BC3

Done: bc3#12386 (`bee714c74`) added the `Recording::VisibleToClientsParam`
concern to `Docks::ToolsController`, gated it on
`tool.changes_client_visibility?` (so only self-visibility tool types honor it),
and documented the parameter in `doc/api/sections/tools.md` (syncs to bc3-api).
Nothing further is required of BC3.

## SDK absorption plan when this lands

Deferred to a follow-up. When absorbed, add optional `visible_to_clients` to the
Smithy `CreateToolInput`, regenerate, add the Go hand-wrapper field + pass-through
(CreateTool has a hand-written wrapper like the content creates), and add a
tri-state transport test asserting explicit `false` reaches the wire — then flip
this entry to `absorbed-in-sdk` with the Smithy ref. Until then it stays
`addressed-in-bc3-pr-12386` as a register-only record.
