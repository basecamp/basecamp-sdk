---
gap: upload-new-version
status: addressed-in-bc3-pr-12555
detected: 2026-07-22
sdk_demand: medium
bc3_pr: 12555
bc3_refs:
  introduced_in: BC3 #12555, inside the range the SDK triaged when it registered this
  routes:
    - POST /:account_id/uploads/:id/versions.json
    - POST /:account_id/buckets/:bucket_id/uploads/:id/versions.json
    - PUT /:account_id/uploads/:id.json
    - GET /:account_id/uploads/:id/versions.json
  controllers:
    - app/controllers/uploads/versions_controller.rb
    - app/controllers/uploads_controller.rb
  related_existing_api:
    - UpdateUpload
    - ListUploadVersions
---

> **Status: shipped in BC3, not yet absorbed by the SDK.** BC3 **#12555**
> ("Expose upload file replacement over the API") added the write side this brief
> asked for. It chose the **second** of the two options proposed below — a
> dedicated `POST /uploads/:id/versions.json` returning **201**, drawn flat and
> bucket-scoped (`resources :versions, only: %i[ index create ], controller:
> "uploads/versions"`) — and **not** the `PUT /uploads/{id}.json` shape
> basecamp-cli#404 hypothesized and the analysis below disproved. The new version
> payload carries a `current` flag among its fields.
>
> It also answers **507** when the account is over its storage allowance. That
> 507 needs **its own error shape**: it is the *storage* limit ("The storage limit
> for this account has been reached.",
> `app/controllers/concerns/resource_limits.rb`), a different resource with a
> different message from the project limit. `ProjectLimitError` — added for
> `CreateProject`/`UnarchiveProject` in the absorption recorded in
> [`project-archive-unarchive.md`](project-archive-unarchive.md) — is **not** it
> and must not be reused.
>
> Absorption belongs to the `upload-versions-api` branch, not to the repin that
> registered this. Until that lands the SDK models **no** operation for these
> routes, which is why `spec/bc3-route-allowlist.yml` carries a
> `registry: spec/api-gaps/upload-new-version.md` disposition for
> `POST /uploads/:id/versions` — one entry, because direction 2 collapses the
> leading `/buckets/:id` and both documented spellings normalize to that key.
>
> Everything below predates #12555 and is preserved as the analysis that
> established what bc3 did *not* do; read it as history, not as current state.

# Upload a new version of an existing file (write side)

## What's missing

There is no JSON API to **replace an existing upload's file** (create a new
version). The read side is fully covered — `GET /uploads/{id}/versions.json`
lists version events and is already modeled as `ListUploadVersions` / absorbed
as `UploadsService.ListVersions` — but nothing writes a version.

basecamp-cli#404 hypothesized that `PUT /uploads/{id}.json` with a fresh
`attachable_sgid` would replace the file and create a version. Verified against
`basecamp/bc3` @ `ba105ba7` (the revision `spec/api-provenance.json` pinned when
this was verified; the pin has advanced several times since) — it does not:

- `UploadsController#update` reads `upload_params`, which permits only
  `:base_name` and `:description`. `attachable_sgid` lives in
  `uploadable_params`, consumed exclusively by `set_new_upload` — a
  `before_action` scoped `only: :create`.
- `wrap_parameters :upload, include: %i[base_name description]` never wraps
  `attachable_sgid` into the params the update reads.
- `Upload#changing` re-attaches the **existing** blob, so
  `track_blob_change` sees an unchanged blob and never records a
  `blob_changed` version event.
- The API route table exposes `versions` as `only: %i[index]` (read-only);
  there is no version-write route.

`PUT /uploads/{id}.json` accepts the key on the wire (strong-params silently
drop it) but takes no action on it. The upload payload permits only
`description` and `base_name`, and no file/blob replacement parameter is
accepted (the update's top-level `status` param changes the recording status,
not the file). Full controller-level evidence:
[`/API-GAP-404.md`](../../API-GAP-404.md).

## Why it matters

Replacing a file in place — keeping the same upload record, its comments, its
URL, and its position, while pushing a new revision — is a common integration
need: synced documents, generated exports refreshed on a schedule, design
iterations. The version *history* is already exposed through the API, which
makes the absence of a version *write* especially visible: a consumer can read
that an upload has five versions but cannot create the sixth. Today the only
way to revise a file through the API is to create a brand-new upload, which
breaks the record's identity, comment thread, and version lineage.

## Suggested API shape

A write contract for file replacement, either:

- **Extend `PUT /uploads/{id}.json`** to honor `attachable_sgid` (and
  optionally `file`), replacing the blob and recording a `blob_changed`
  version event; or
- **Add a dedicated version-create route**, e.g.
  `POST /uploads/{id}/versions.json` accepting `attachable_sgid`, mirroring the
  create-upload contract and returning the updated upload (or the new version
  event).

Either way the request carries an `attachable_sgid` obtained from the Create
Attachment endpoint, exactly as create-upload does, and the response should
reflect the new blob (`byte_size`, `content_type`, `filename`, `download_url`).

## Implementation notes for BC3

- Decide the surface: widen `update`'s permitted params to include
  `attachable_sgid`/`file` and thread them through `@upload.changing`, or add a
  version-create action. A metadata-only update must remain a no-op on the
  blob so it does not spuriously create versions.
- Ensure the replacement path drives `track_blob_change` so a new
  `blob_changed` event is recorded and surfaces in
  `GET /uploads/{id}/versions.json`.
- Document the chosen route in `doc/api/sections/uploads.md`, including whether
  `description`/`base_name` may accompany the replacement.

## SDK absorption plan when this lands

- Model the write operation in `spec/basecamp.smithy` (extend
  `UpdateUploadInput` with `attachable_sgid`, or add a `CreateUploadVersion`
  operation), then `make smithy-build` and regenerate.
- Map the new field/operation in `UploadsService` (`go/pkg/basecamp/vaults.go`)
  and the peer SDKs; add `AttachableSGID` to `UpdateUploadRequest` only once the
  server honors it.
- Add a canary fixture exercising a real file replacement and confirm the
  version list grows, against a live account, before the CLI (basecamp-cli#404)
  exposes an "upload new version" command.
- Replace the reflection guard
  (`TestUpdateUploadRequest_HasNoFileReplacementField`) with a positive
  assertion that the field is now sent.
