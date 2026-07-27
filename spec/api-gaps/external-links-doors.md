---
gap: external-links-doors
status: partial-coverage
detected: 2026-07-22
sdk_demand: low
bc3_pr: 12375
smithy_refs:
  - "Door in the documented recording type string (spec/basecamp.smithy:6223)"
  - "Recording.position/description/service (spec/basecamp.smithy:6282-6293)"
  - "DoorService (spec/basecamp.smithy:6309)"
bc3_refs:
  routes:
    - GET /:account_id/projects/recordings.json?type=Door
    - POST /:account_id/buckets/:bucket_id/dock/doors.json
    - GET /:account_id/dock/tools/:id.json
    - PUT /:account_id/dock/tools/:id.json
    - DELETE /:account_id/dock/tools/:id.json
  controllers:
    - app/controllers/docks/doors_controller.rb
    - app/controllers/docks/tools_controller.rb
    - app/controllers/recordings_controller.rb
  related_existing_api:
    - ListRecordings
    - GetTool
    - UpdateTool
    - DeleteTool
---

# External links (doors) API surface

## What's missing

BC3 **#12375** ("Add API documentation for External links (doors)", merged
`60a7f598`, 2026-07-22) newly documents the external-link (historically "door")
resource — a dock tool that points to an outside URL (Figma, Dropbox, GitHub,
etc.). New `doc/api/sections/external_links.md` documents:

- **List** — `GET /:account_id/projects/recordings.json?type=Door` (the canonical
  enumeration and the only endpoint returning the full door shape: `url`,
  `service` struct, `description`). Accepts the generic recordings `bucket`,
  `status`, `sort`, `direction` params.
- **Create** — `POST /:account_id/buckets/:bucket_id/dock/doors.json`. Bespoke path;
  returns **302** (redirect), no created-resource JSON body / no ID in the
  response.
- **Get / Rename / Trash** — via the generic dock-tool operations at
  `/:account_id/dock/tools/:id.json`: `GET` (get), `PUT` with a `title` (rename), and
  `DELETE` (trash — soft-deletes the door, `status` → `"deleted"`). The legacy
  `DELETE /:account_id/buckets/:bucket_id/dock/doors/:id.json` is an alias.
- **No JSON update path** for `url`/`service`/`description` — a `PUT` to the
  tool returns **406**; changes go through the HTML redirector only.

The same PR adds `Door` to the documented `type` enum for
`GET /:account_id/projects/recordings.json` in `recordings.md`.

## Relationship to existing entries

This **supersedes the "Door is string-only" classification** in
[[recordable-subtypes-doc]], which stated Door "appears only as a string `type`
value" with no create/list surface. That is now stale: #12375 documents a
door-specific list endpoint (full shape), a bespoke create path, and dock-tool
get/rename/trash. Absorption should be tracked here, not there. Door as a
`RecordingType` enum member is also a doc-string delta on the existing
`ListRecordings` output.

## Why it matters

SDK consumers cannot currently enumerate a project's external links (the shape
is only returned by the `type=Door` recordings query) or create one through a
typed operation. Demand is low — external links are a legacy dock surface — but
the contract is now documented and must be tracked to keep detection honest.

## Suggested API shape

- A `type=Door`-scoped recordings list (or dedicated `ListExternalLinks`)
  returning the full door shape: `url`, `service` struct, `description`.
- A create operation that honors the 302/no-body contract (returns no
  resource JSON).
- Reuse the existing dock-tool GetTool/UpdateTool/DeleteTool operations for
  get/rename/trash.
- `Door` added to the `RecordingType` documented enum.

## Implementation notes for BC3

- Already merged in BC3 #12375 (docs). Endpoint URLs keep the legacy `doors`
  resource name.
- External links are deliberately omitted from a project's `dock` array, so
  the "discover via dock" advice does not surface them — the `type=Door` list
  is the canonical enumeration.
- There is no JSON update path for `url`/`service`/`description` (PUT → 406);
  updates go through the HTML redirector only.

## SDK absorption plan when this lands

**Partially absorbed** (basecamp-sdk PR-4 of the post-#401 follow-up program).
A pre-implementation spike resolved the three blockers and scoped this PR to the
cleanly-absorbable surface; the redirect-dependent create and the multipart
image are recorded as residual gaps below.

**Absorbed in this PR:**

- The door **list** is the existing `type=Door`-scoped `ListRecordings` query —
  **not** a new operation. `ListRecordings` already carries a `type` query and a
  `Recording.url`; the full door shape is reached by adding `Door` to the
  `RecordingType` documented enum and adding optional `position`, `description`,
  and a `service` struct (`DoorService`) to the shared `Recording` projection.
  A runtime decode test proves the door shape (external `url` + `service` +
  `description`) decodes through the projection.
- Get/rename/trash reuse the existing `GetTool` / `UpdateTool` / `DeleteTool`
  operations — no new work.
- A `type=Door` recordings canary entry validates statically (live dormant).

**Residual gaps (why this entry is `partial-coverage`, not
`absorbed-in-sdk`):**

- **Create (`POST /buckets/:b/dock/doors.json`) — deferred.** The endpoint
  returns **302 with an empty body** (no ID), redirecting cross-origin to the
  web project page. The spike found the six SDK transports handle this
  inconsistently: Go, TypeScript (fetch default), Kotlin (Ktor default), and
  Swift (URLSession default) **follow** the redirect (landing on the web page,
  after cross-origin auth stripping); Python (`follow_redirects=False`) and Ruby
  (Faraday, no follow middleware) do **not** follow but then face empty-body
  decode, and each classifies a bare 302 differently. Modeling "return only the
  302/empty response" would require coordinated per-operation redirect
  suppression + 302-as-success handling across all six generated transports — a
  cross-cutting transport change, not an additive absorption. Deferred to a
  dedicated PR.
- **`image` thumbnail (multipart) — unmodeled.** `door[image]` is
  `multipart/form-data` only; not modeled. Independent of the create-transport
  question, this alone keeps the entry from honestly flipping to
  `absorbed-in-sdk`.
- **Create-and-discover composite (SPEC §18) — deferred.** It depends on a
  shippable `CreateExternalLink`, so it is out of scope until create lands.
- **No JSON update path** for `url`/`service`/`description` (PUT → 406): never
  modeled, by contract.

## Compatibility

Mostly additive, with one deliberate optionality **correction** — not purely
additive. No new operation and no change to existing modeled operations.

- **Additive:** the `Door` recording type value plus the optional
  `position`/`description`/`service` fields on the shared `Recording` output.
- **Optionality correction (`Recording.parent`):** this entry relaxes
  `Recording.parent` from required to optional. That changes the generated
  public types (Swift/Kotlin/TypeScript/Python surface `parent` as optional), so
  it is technically source-affecting for consumers that assumed `parent` is
  always present. It is nonetheless a **fix of a pre-existing contract defect,
  not a gratuitous break**: BC3's shared recording projection
  (`app/views/api/recordings/_recording.json.jbuilder`) emits `parent` only
  `if !recording.docked? && recording.parent`, so it is **omitted for every
  docked recording** — message boards, to-do sets, schedules, campfires,
  questionnaires, and doors (all dock items, `docked? == parent&.dock?`) — and
  for any parentless recording. The prior `@required` therefore over-asserted a
  field the API has always emitted conditionally, and strict decoders
  (Swift/Kotlin) would already have failed to decode any dock item. Door (a
  docked recording) only made the latent defect unavoidable. Because it changes
  a public type, it should still be called out in the release notes as a
  compatibility-affecting correction.
