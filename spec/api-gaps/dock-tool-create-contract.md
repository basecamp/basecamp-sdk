---
gap: dock-tool-create-contract
status: absorbed-in-sdk
detected: 2026-05-25
sdk_demand: medium
smithy_refs:
  - "CreateTool (spec/basecamp.smithy:6874)"
bc3_refs:
  introduced_in: five
  routes:
    - "POST /:account_id/buckets/:bucket_id/dock/tools.json"
  controllers:
    - app/controllers/docks/tools_controller.rb
  related_existing_api:
    - GetTool
    - UpdateTool
---

# Dock tool creation changed contract BC4→BC5 (clone → create-by-type)

> **Not an additive gap.** Like [memories-emptied-regression](memories-emptied-regression.md),
> this entry records a live BC4→BC5 *contract change* on an existing route,
> not new BC5 surface awaiting coverage. `absorbed-in-sdk` here means the SDK
> absorbed the new create-by-type contract (basecamp-sdk#327, merged
> 2026-07-22); upstream documentation still describes the removed contract
> (tracked as bc3#12364).

## What's missing

`POST /:account_id/buckets/:bucket_id/dock/tools.json` creates a dock tool,
but the request contract flipped between BC4 and BC5:

- **BC4 (`four`, tracked by `compatibility.bc3-four` provenance):**
  clone-shaped. `Docks::ToolsController` runs an unconditional
  `set_source_recording` with `params.require(:source_recording_id)` — the
  new tool is cloned from an existing source tool. A request without
  `source_recording_id` fails with **400**.
- **BC5 (`master`):** create-by-type. The `five` branch dropped the
  `source_recording_id` requirement (`477049c0d7`, 2026-03-18) and added
  creation from `tool_type` + optional `title` (`c1a59bf1a9`, 2026-04-23);
  `five` merged to `master` in `1d08f11882` at 2026-05-25T23:36:59Z. Valid
  `tool_type` values are `Dock::DefaultDockables`: `Chat::Transcript`,
  `Inbox`, `Kanban::Board`, `Message::Board`, `Questionnaire`, `Schedule`,
  `Todoset`, `Vault`. On current `master` a clone-shaped bucket-scoped body
  (`source_recording_id` + `title`, no `tool_type`) **404s**: with
  `bucket_id` present the source-recording lookup is skipped, so the tool
  type resolves to nothing and the `Dock::DefaultDockables` fetch raises
  `RecordNotFound`. basecamp-cli#471 (filed 2026-05-25T09:22:54Z, ~14h
  before the `five` merge landed) evidences only the *flat-route* SDK
  artifact: the legacy SDK's `POST /{account}/dock/tools.json` 404'd while
  the reporter's raw bucket-scoped clone request still succeeded — the
  reporter caught the contract mid-flip.

What's missing, precisely: upstream `doc/api/sections/tools.md` (canonical
since repatriation `056a356ee0`; synced to bc3-api) still documents the
removed clone contract. The compatibility posture itself is settled by
release: **BC5 replaced BC4 in production, so there is no live BC4 backend**
— `four` survives as the wire-format reference (`compatibility.bc3-four`)
for BC4-era clients, not as a server anyone can call.

## The precise compatibility gap

A **hybrid wire body** (`source_recording_id` + `tool_type` + `title`) is
structurally accepted by both snapshots: BC4 consumes the source ID and
ignores `tool_type`; `master` consumes `tool_type` and skips the source
lookup. So a cross-version wire body *exists* — but the SDK's new
create-shaped signature (basecamp-sdk#327: `tool_type` + optional `title`,
no source-ID parameter) **cannot emit it**. The gap is therefore:

> **The new SDK `CreateTool` signature is not BC4-compatible** — against the
> `four` contract it fails on the missing `source_recording_id` (400) — not
> "no cross-version wire body exists".

The failure mode is body-level only: the bucket-scoped route itself exists on
**both** backends (`resource :dock { resources :tools }` under
`resources :buckets` on `four` as well as `master`), routing to the same
controller. The legacy SDK's flat `POST /{accountId}/dock/tools.json` model
was an SDK artifact, not a BC4 requirement — under the `four` contract the
bucket-scoped route fails on `params.require(:source_recording_id)` (400),
never on the URL.

Since BC5 replaced BC4 at release, this direction (new SDK → BC4 server) is
a wire-format-reference statement only. The **live** direction is the
reverse: BC4-era clients still emitting the clone shape fail against
production — the legacy flat path 404s (what basecamp-cli#471 observed), and
a bucket-scoped clone body without `tool_type` 404s via the skipped source
lookup described above.

## Why it matters

The route answers on both backends, so per-backend schema validation passes
everywhere; only a pairwise BC4↔BC5 comparison (or a live failure) surfaces
the divergence. BC4-era clients written to the documented clone contract
break at request time against production, not at build time — the same
visibility class as
[memories-emptied-regression](memories-emptied-regression.md), on the write
path instead of the read path.

## Suggested API shape

None new — `master` already decided the shape: `{tool_type, title?}` → 201
with the created tool, bucket-scoped. Compatibility posture is settled by
release (no live BC4 backend); the remaining tail is documentation.

## Implementation notes for BC3

One upstream item, standalone (BC3 #11628/#11629 are merged, so this is a
new follow-up, not an addition to that train): rewrite the create section of
`doc/api/sections/tools.md` for `tool_type` + optional `title` (syncs to
bc3-api). The current text documents a contract that 404s on `master` —
tracked as bc3#12364. (A `four` compatibility shim is moot: BC5 replaced BC4
at release, so there is no live BC4 backend to shim.)

## SDK absorption plan when this lands

Absorbed: basecamp-sdk#327 (CloneTool → CreateTool rename, bucket-scoped
endpoint, `{tool_type, title?}` body, per-SDK create signatures + tests +
conformance) merged 2026-07-22. The only remaining tail is the upstream doc
rewrite (bc3#12364); no further SDK change is needed.
