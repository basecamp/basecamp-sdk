---
gap: external-links-doors
status: addressed-in-bc3-pr-12375
detected: 2026-07-22
sdk_demand: low
bc3_pr: 12375
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

- Model the door **list** as a `type=Door`-scoped recordings query (or a
  dedicated `ListExternalLinks` operation) returning the full door shape
  (`url`, `service`, `description`).
- Model **create** carefully: the 302/no-JSON-body contract does not fit the
  standard "create returns the resource" mold — the operation returns no
  usable body, so the SDK surface must not promise one.
- Get/rename/trash reuse the existing dock-tool GetTool/UpdateTool/DeleteTool
  operations.
- Add `Door` to the `RecordingType` documented enum.

## Compatibility

New documented resource surface; no change to existing modeled operations
beyond the additive `Door` enum value on `ListRecordings` output type.
