---
gap: template-library
status: absorbed-in-sdk
detected: 2026-09-02
sdk_demand: high
bc3_pr: 12953
smithy_refs:
  - GetTemplateLibrary
  - CreateTemplateLibraryCopy
  - GetTemplateLibraryCopy
bc3_refs:
  introduced_in: "Add JSON API for the to-do list template library (BC3 #12953, merged 58911921e7b)"
  routes:
    - GET /:account_id/template_library.json
    - POST /:account_id/template_library/copies.json
    - GET /:account_id/template_library/copies/:id.json
  controllers:
    - app/controllers/template_libraries_controller.rb
    - app/controllers/template_library/copies_controller.rb
  related_existing_api:
    - Todolist
    - Todoset
    - RecordingBucket
---

# To-do list template library

## What's missing

Nothing remains missing from the documented template-library API. An account
exposes one shared library of reusable to-do list templates.
`GET /template_library.json` returns the library bucket, its to-do set, and the
library's to-do lists. The operation uses the standard recording projections,
so callers receive the same `Todolist`, `RecordingParent`, and
`RecordingBucket` shapes used by project to-do lists.

Copying is asynchronous. A copy resource reports `pending`, `processing`,
`completed`, or `failed` status, and a completed copy includes
`destination_todolist`. Templates can reference people who do not yet have
access to the destination project. The create operation returns `422` with the
people requiring approval; explicit confirmation grants access and starts the
copy.

## Why it matters

The library provides a single account-wide source for repeatable project
workflows. SDK clients can browse approved templates, copy one into an existing
project, explicitly confirm any access grants, and follow the asynchronous copy
to completion.

## Suggested API shape

The deployed API provides three operations on the Templates service:

- `GetTemplateLibrary`: `GET /template_library.json` reads the account library.
- `CreateTemplateLibraryCopy`: `POST /template_library/copies.json` accepts
  `template_recording_id`, `destination_parent_id`, and optional
  `adding_people_confirmed`, then returns the copy with `201`.
- `GetTemplateLibraryCopy`: `GET /template_library/copies/{copyId}.json` reads
  copy status and the completed destination list. Only the person who started a
  copy can retrieve it; other callers receive `404`.

## Implementation notes for BC3

BC3 #12953 renders the library from `TemplateLibrariesController` and copy
resources from `TemplateLibrary::CopiesController`. The copy endpoint returns
`PeopleConfirmationRequiredError` with `error` and the confirmation person's
`id`, `name`, and `avatar_url`. Polling uses the URL returned by create and the
same credentials that started the copy.

## SDK absorption plan when this lands

Absorbed in all six SDKs. Smithy owns the three routes, request and response
structures, copy status values, and `PeopleConfirmationRequiredError`.
Generated service layers and shared conformance cases pin all three paths,
request field names, the `201` response, the completed-copy projection, and the
specialized `422` validation error with its typed confirmation people. Go's `TemplatesService` provides `GetLibrary`,
`CreateLibraryCopy`, and `GetLibraryCopy` convenience methods over generated
operations.
