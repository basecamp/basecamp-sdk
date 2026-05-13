---
gap: recordable-subtypes-doc
status: partial-coverage
detected: 2026-05-01
sdk_demand: medium
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3a
  routes:
    - "POST /:account_id/buckets/:bucket_id/journals.json (create)"
    - "POST /:account_id/buckets/:bucket_id/journals/:journal_id/entries.json (create)"
    - "POST /:account_id/buckets/:bucket_id/cloud_files.json (create)"
    - "POST /:account_id/buckets/:bucket_id/google_documents.json (create)"
    - "(plus polymorphic surfacing in existing Recording responses — no new GET routes)"
  controllers:
    - app/controllers/journals_controller.rb
    - app/controllers/journals/entries_controller.rb
    - app/controllers/cloud_files_controller.rb
    - app/controllers/google_documents_controller.rb
  related_existing_api:
    - GetRecording (polymorphic by recording_type)
---

# Recordable subtypes — doc + Smithy ops + create

## What's missing

BC5 surfaces several Recording subtypes that have always existed in the
codebase but have never been formally documented as API-shaped:

- `Journal` and `Journal::Entry` (per-project journal posts)
- `CloudFile` (linked external file: Google Drive, Dropbox, etc.)
- `GoogleDocument` (specifically a Google Docs link, distinct from CloudFile)

These appear polymorphically in existing Recording responses (search results,
activity feeds, etc.). Their wire shapes likely render via existing partials,
but the subtypes are absent from `doc/api/` and from the SDK's typed shapes.

The BC3 plan Phase 3a covers documentation + Smithy shapes **and `create`
operations** for all four subtypes (Journal, Journal::Entry, CloudFile,
GoogleDocument). This is not shape/doc-only absorption — the SDK adds
`Create*` operations to match.

**Door is excluded** by the BC3 plan from standalone API coverage and is
covered (if at all) only as a string-typed `type` value on existing
Recording responses.

## Why it matters

SDK consumers iterating through Recording-shaped responses (Search,
Activity, etc.) currently see opaque payloads when the underlying Recording
is one of these subtypes. Adding typed shapes lets them pattern-match on
`type` and render appropriately.

## Suggested API shape

Two parallel additions — documentation/typed shapes for read paths, and new
POST endpoints for create paths.

**Read paths (already surface via polymorphism):**

1. Document the wire shape for each subtype:
   - `Journal`: id, title, status, urls, recordings_count, etc. (Recording-like)
   - `Journal::Entry`: id, title, content, journal (parent reference)
   - `CloudFile`: id, name, content_type, external_url, source ("Google Drive" etc.)
   - `GoogleDocument`: id, name, google_url, doc_type
2. Confirm `type` value emitted by each (e.g. `"Journal::Entry"`,
   `"CloudFile"`, `"GoogleDocument"`).
3. Verify polymorphic Recording-shaped responses include these without
   manual whitelisting (`Recording#bubbleupable?` test should suffice).

**Create paths (BC3 Phase 3a):**

- `POST /:account_id/buckets/:bucket_id/journals.json` — input: `title`,
  optional `description`/`status`. Returns the created Journal.
- `POST /:account_id/buckets/:bucket_id/journals/:journal_id/entries.json`
  — input: `title`, `content`. Returns the created Journal::Entry.
- `POST /:account_id/buckets/:bucket_id/cloud_files.json` — input: external
  URL, name, source. Returns the created CloudFile.
- `POST /:account_id/buckets/:bucket_id/google_documents.json` — input:
  Google Doc URL, name. Returns the created GoogleDocument.

Confirm the exact controller paths and input shapes against BC3 Phase 3a
before opening the absorption PR.

## Implementation notes for BC3

- Spot-check `app/views/api/journals/`, `app/views/api/cloud_files/`,
  `app/views/api/google_documents/` — partials may already exist via
  polymorphic dispatch.
- Add `doc/api/sections/journals.md`, etc.
- For create endpoints: confirm controller-level routes; existing
  `journals_controller.rb`, `cloud_files_controller.rb`, etc. likely host
  the `create` action with a JSON branch added in Phase 3a.

## SDK absorption plan when this lands

- New Smithy structures: `Journal`, `JournalEntry`, `CloudFile`,
  `GoogleDocument`.
- New Smithy operations: `CreateJournal`, `CreateJournalEntry`,
  `CreateCloudFile`, `CreateGoogleDocument` — each with input/output
  structures.
- New service registrations: likely `JournalsService`, `CloudFilesService`,
  `GoogleDocumentsService` (consolidate if BC3 puts them under a single
  controller namespace).
- Where Recording is polymorphic, extend the discriminated structure (or
  add new union members) with the new `type` values.
- Door is not modelled separately — appears as a string `type` value on
  existing Recording responses, undecoded.
- Canary: extend the `Search` and `Activity` fixture coverage to include
  recordings of these new types if test data permits. Create operations
  not covered by canary (read-only canary scope).
