---
gap: upload-create-subscriptions
status: partial-coverage
detected: 2026-08-05
sdk_demand: low
bc3_refs:
  routes:
    - POST /:account_id/vaults/:vault_id/uploads.json
    - POST /:account_id/vaults/:vault_id/uploads/publish.json
  controllers:
    - app/controllers/uploads_controller.rb
    - app/controllers/vaults/uploads_controller.rb
  related_existing_api:
    - CreateUpload
    - CreateUploadVersion
---

# `CreateUploadInput.subscriptions` is modeled but never consumed

## What's missing

Nothing is missing from BC3 — the defect is on the SDK side. `CreateUploadInput`
models an input the endpoint ignores, so what's missing is either the server
behaviour the field promises or the field's removal. Details below.

## What's wrong

`CreateUploadInput` declares `subscriptions: PersonIdList`
(`spec/basecamp.smithy`), so every SDK offers it on create. The endpoint ignores
it.

`CreateUpload` is `POST /{accountId}/vaults/{vaultId}/uploads.json`, which
`config/routes.rb` routes to `UploadsController#create`. That controller does
**not** include the `Subscribers` concern, and its `create` calls
`@bucket.record(@new_upload.recordable, parent:, status:, visible_to_clients:)`
— no subscribers argument anywhere in the path.

`subscriptions` *is* consumed for uploads, but by a different controller on a
different route: `Vaults::UploadsController#publish`
(`POST /vaults/:vault_id/uploads/publish.json`), which passes
`subscribers: find_subscribers` into `Upload::Publisher.publish`. An API-created
upload never reaches it — `Upload::Creation#status` returns `active` for an
sgid-backed upload, so there is no draft left to publish.

`doc/api/sections/uploads.md` is consistent with the code: the "Create an upload"
section documents `attachable_sgid`, `description`, `base_name` and
`visible_to_clients`, and never mentions `subscriptions`.

## Why it matters

This is the same class of defect as basecamp-sdk#649, in the structure right
next to the one that fixed it. A caller who passes `subscriptions` to
`uploads.create` gets no error and no subscribers — the field is accepted,
serialized, sent, and dropped. Silent no-ops are worse than absent features,
because nothing tells the caller to go looking.

It is scoped `low` only because the workaround is easy once known
(`SubscriptionsService` after the create), not because the lie is mild.

## Contrast with `CreateUploadVersion`

`CreateUploadVersion` models `notify` and `subscriptions` and both are live —
`Uploads::VersionsController` includes `Subscribers`, and basecamp/bc3#12565
documented and tested all four input shapes. So the two operations in the same
service now disagree about whether `subscriptions` does anything, which is
exactly the mixed-shape footgun to resolve rather than leave.

## Suggested API shape

Either the field goes, or BC3 grows the behaviour it names:

1. **Remove `subscriptions` from `CreateUploadInput`.** Honest, and a breaking
   change to the generated surface in six SDKs — needs its own `MIGRATING.md`
   entry. Preferred.
2. **Make BC3 honor it** by including `Subscribers` in `UploadsController#create`
   and threading `find_subscribers` through, matching the versions controller.
   Turns the lie into a feature, and would make create consistent with
   `CreateUploadVersion`, where both `notify` and `subscriptions` are live.

## Implementation notes for BC3

Only needed if option 2 is chosen. `UploadsController#create` would include
`Subscribers` and pass `subscribers: find_subscribers` into the record call, and
`doc/api/sections/uploads.md` would gain `notify` and `subscriptions` bullets on
"Create an upload" mirroring the ones basecamp/bc3#12565 added to "Create an
upload version". Note the upload is already `active` at create for an
sgid-backed upload, so the publish path is not where this would live.

If option 1 is chosen, BC3 needs no change at all — the docs are already correct.

## SDK absorption plan when this lands

Deliberately not bundled into the `CreateUploadVersion` absorption: removing an
existing input member is breaking, and belongs in a change whose title says so.

- Option 1: drop the member from `CreateUploadInput`, regenerate, and add a
  `MIGRATING.md` breaking entry alongside the other uploads-surface entries.
  Check the CLI and MCP server for callers first — a silent no-op has no
  compile-time users to find, so grep rather than trusting the build.
- Option 2: nothing to change in the SDK; the field starts working, and this
  entry closes as `absorbed-in-sdk` with a test asserting subscribers land.
