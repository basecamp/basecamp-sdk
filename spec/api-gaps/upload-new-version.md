---
gap: upload-new-version
status: absorbed-in-sdk
detected: 2026-07-22
sdk_demand: medium
bc3_pr: 12555
smithy_refs:
  - CreateUploadVersion
  - CreateUploadVersionInput
  - UploadVersion
  - UploadVersionFile
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

**Done.** Landed in basecamp/bc3#12555 + #12565 and absorbed here; the plan below
is kept as the record of what was decided.

BC3 took the **second** option in "Suggested API shape": a dedicated
`POST /uploads/{id}/versions.json`, shipped in basecamp/bc3#12555. `PUT
/uploads/{id}.json` was deliberately left alone, so the hypothesis
basecamp-cli#404 started from stays false — the update still permits only
`base_name` and `description`.

That decision inverts one bullet of the old absorption plan. The reflection
guard `TestUpdateUploadRequest_HasNoFileReplacementField` is **not** replaced
with "the field is now sent"; it is still true and still worth holding, because
it now pins a design choice rather than a missing feature. It keeps its name,
gains a comment pointing at the sanctioned path, and is joined by a positive
counterpart asserting `CreateUploadVersionRequest` carries `AttachableSGID`.

Absorbed as:

- `CreateUploadVersion` — the write, tagged `Files`, grouped into `Uploads`.
- `UploadVersion` / `UploadVersionFile` — the read side. `ListUploadVersionsOutput`
  previously declared `uploads: UploadList`, which was a typed lie: the endpoint
  returns *events*, and 11 of `Upload`'s 14 `@required` members are absent from
  every response. That is basecamp-sdk#649, fixed here rather than merely
  corrected, because #12555 also added the nested `upload` object that gives the
  version list a filename to report.
- `StorageLimitError` — the `507 Insufficient Storage` contract, which
  `ensure_account_can_upload_files` also fronts on three operations the SDK
  already modeled (`CreateUpload`, `CreateAttachment`, `CreateCampfireUpload`).
  All four now declare it, and SPEC §6 maps 507 to `limit_exceeded` /
  non-retryable instead of letting it fall through to the retryable 5xx
  catch-all.

Input contract settled in basecamp/bc3#12565: `notify` and `subscriptions` are
documented and tested, and `visible_to_clients` was removed from the endpoint's
reachable surface (it never set visibility — it only widened the notification
audience, and could announce a client-invisible file to a project's clients).

The live-account canary ran 2026-08-11 against production (account 2914079,
Coworker QA Sandbox vault) via a throwaway Go program built on this repo's
`go/` module: `CreateVersion` kept the upload's id and URL while reflecting the
replacement's filename/byte_size/content_type; `ListVersions` decoded as typed
`[]UploadVersion` with exactly one `Current` entry and `blob_changed` as the
newest action; the older version's `download_url` served the original bytes and
`Download` served the replacement's (both byte-compared); a `nil` description
carried the create-time description forward. All assertions passed.

Still open: basecamp-cli#404's "upload new version" command.
