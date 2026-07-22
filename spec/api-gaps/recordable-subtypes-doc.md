---
gap: recordable-subtypes-doc
status: partial-coverage
detected: 2026-05-01
sdk_demand: medium
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3a
  routes:
    - "POST /:account_id/buckets/:bucket_id/cloud_files.json (create — shipped)"
    - "POST /:account_id/buckets/:bucket_id/google_documents.json (create — shipped)"
    - "(Journal / Journal::Entry routes — NOT shipped; dropped from the BC5 API train)"
    - "(plus polymorphic surfacing in existing Recording responses — no new GET routes)"
  controllers:
    - app/controllers/cloud_files_controller.rb
    - app/controllers/google_documents_controller.rb
    - app/controllers/journals_controller.rb
    - app/controllers/journals/entries_controller.rb
  related_existing_api:
    - GetRecording (polymorphic by recording_type)
---

# Recordable subtypes — doc + Smithy ops + create

## What's missing

Split outcome from the BC5 API train (2026-07-18..21), tracked in this single
brief (no split into per-subtype briefs):

- **Shipped:** `CloudFile` (linked external file: Google Drive, Dropbox, etc.)
  and `GoogleDocument` (specifically a Google Docs link, distinct from
  CloudFile) landed via BC3 **#12320** — `doc/api/sections/cloud_files.md` and
  `doc/api/sections/google_documents.md` are on `master`. SDK absorption for
  these two is pending.
- **Did NOT ship:** `Journal` and `Journal::Entry`. BC3 **#11629**'s "Drop
  journal doc generation" commit removed the journal JSON API coverage from
  the train — no product surface, no traffic. The journal routes remain
  undocumented; treat them as out of API scope until BC3 revisits.

The status stays `partial-coverage`: part of the subtype family has a merged
contract awaiting absorption, part has no JSON contract at all.

**Door remains excluded** from standalone API coverage and is covered (if at
all) only as a string-typed `type` value on existing Recording responses.

## Why it matters

SDK consumers iterating through Recording-shaped responses (Search, Activity,
etc.) currently see opaque payloads when the underlying Recording is one of
these subtypes. Adding typed shapes lets them pattern-match on `type` and
render appropriately.

## Suggested API shape

For the shipped pair, follow the merged docs:

- `CloudFile` — per `doc/api/sections/cloud_files.md` (create route plus the
  documented wire shape).
- `GoogleDocument` — per `doc/api/sections/google_documents.md` (create route
  plus the documented wire shape).

Confirm the exact `type` discriminator values against the merged docs'
captured examples at absorption time. No shape is proposed for Journal /
Journal::Entry — there is no contract to model.

## Implementation notes for BC3

- CloudFile + GoogleDocument: shipped — nothing pending.
- Journal: if BC3 later gives journals a product surface and JSON contract,
  that is net-new API work (routes, jbuilder views, doc section); this brief
  then updates its scope.

## SDK absorption plan when this lands

- New Smithy structures: `CloudFile`, `GoogleDocument`; new operations:
  `CreateCloudFile`, `CreateGoogleDocument` — inputs/outputs per the merged
  docs.
- New service registrations: `CloudFilesService`, `GoogleDocumentsService`
  (or consolidated, matching BC3's controller namespacing).
- No `Journal` / `JournalEntry` shapes or operations — dropped from the train;
  do not model ahead of a BC3 contract.
- Where Recording is polymorphic, extend the discriminated structure with the
  new `type` values.
- Door is not modelled separately — appears as a string `type` value on
  existing Recording responses, undecoded.
- Canary: extend the `Search` and `Activity` fixture coverage to include
  recordings of these new types if test data permits. Create operations not
  covered by canary (read-only canary scope).
