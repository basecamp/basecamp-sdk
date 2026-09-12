---
gap: template-library-card-tables
status: partial-coverage
detected: 2026-09-11
sdk_demand: high
smithy_refs:
  - GetTemplateLibrary
  - GetTemplateLibraryTodolists
  - GetTemplateLibraryCardTables
  - CreateTemplateLibraryCardTable
  - CreateTemplateLibraryTodolist
  - CreateTemplatification
  - GetTemplatification
  - CreateTemplateLibraryCopy
  - GetTemplateLibraryCopy
  - TemplateLibraryTodolists
  - TemplateLibraryCardTables
  - TemplateLibraryCopy
  - Templatification
  - CardTable
  - Project
bc3_refs:
  introduced_in: "bc3 branch template-cli-api: route move (48489ed5c8), card table templates through the API (0ecc540a12), alias re-documentation (49e616964d), and the to-do list create plus templatification docs (37a19aea42)"
  routes:
    - GET /:account_id/template_library/todolists.json
    - GET /:account_id/template_library/card_tables.json
    - POST /:account_id/template_library/card_tables.json
    - POST /:account_id/template_library/todolists.json
    - POST /:account_id/buckets/:bucket_id/recordings/:recording_id/templatifications.json
    - GET /:account_id/buckets/:bucket_id/recordings/:recording_id/templatifications/:id.json
    - POST /:account_id/template_library/copies.json
    - GET /:account_id/template_library/copies/:id.json
  controllers:
    - app/controllers/template_library/todolists_controller.rb
    - app/controllers/template_library/card_tables_controller.rb
    - app/controllers/template_library/copies_controller.rb
    - app/controllers/recordings/templatifications_controller.rb
    - app/controllers/template_libraries_controller.rb
  related_existing_api:
    - CardTable
    - Todolist
    - Project
    - Recording
---

# Card table templates, and the per-kind template library

## What's missing

The account's template library holds three kinds of template. Only to-do lists
had a JSON API. [`template-library.md`](template-library.md) records that
earlier absorption and stays as the record of the to-do-list-only contract.

BC5 has shipped card table templates in the product, and the library's single
URL predated the split into kinds: every address under `template_library`
silently meant "to-do list templates". A caller could neither list card table
templates nor create one.

## Why it matters

Card table templates are the half of the feature a script cannot reach. A CLI
can already browse and copy to-do list templates; without these routes it cannot
show a user the card table templates that sit beside them in the same library,
which reads as the feature being broken rather than partial.

## Suggested API shape

Five operations, four of them on the Templates service plus one project field:

- `GetTemplateLibraryTodolists`: `GET /template_library/todolists.json`, the
  kind-explicit address for the existing library read.
- `GetTemplateLibrary`: `GET /template_library.json`, retained and deprecated.
  bc3 still serves and documents it as an alias that 302s to the to-do list
  index and carries the requested format across the redirect.
- `GetTemplateLibraryCardTables`: `GET /template_library/card_tables.json`
  returns the bucket, the `Kanban::Boardset` container, and the active card
  table templates in title order. `kanban_boardset` is `null` for a library that
  has never held one, with `card_tables` empty.
- `CreateTemplateLibraryCardTable`: `POST /template_library/card_tables.json`
  takes a write-only `name` and returns `201` with the new card table.
- `CreateTemplateLibraryCopy` / `GetTemplateLibraryCopy`: unchanged addresses.
  One endpoint serves both kinds; a completed copy reports
  `destination_card_table` beside the existing `destination_todolist`.
- `CreateTemplateLibraryCopy` gains an optional `destination_project_id`.
  Basecamp resolves the container from the template's kind, so a caller names a
  project rather than a dock or to-do set.

## Implementation notes for BC3

The copy endpoint resolves the container from the template's kind when given a
`destination_project_id`: a to-do list template copies to the project's to-do
set, a card table template to its dock. `destination_parent_id` remains for a
caller that already holds a container id. `set_destination_parent` accepts only `Dock` and `Todoset` recordings
and raises `RecordNotFound`, so a caller who cannot edit the destination project
sees `404` rather than `403`, matching how recording lookups fail elsewhere.

A card table template hangs off the library's boardset rather than the dock, so
the shared recording projection emits `parent` and omits `position`; a project
card table is the mirror image. One `CardTable` shape covers both.

## SDK absorption plan when this lands

Absorbed in all seven SDKs ahead of the upstream merge, which is why this entry
is `partial-coverage` rather than `absorbed-in-sdk`: the contract exists on the
unmerged bc3 branch `template-cli-api` and is not yet servable in production, so
`make bc3-route-parity` reports six route/method pairs across four paths as
undocumented by bc3 at the current provenance pin. That failure is expected and is the whole reason
this entry exists. It clears at the repin that follows the upstream merge, with
no SDK change: the routes enter `spec/bc3-routes.json` from bc3's own
`doc/api/sections/`, which already carries the bullets and markers for them.

Flip this entry to `absorbed-in-sdk` at that repin, and record the bc3 PR number
in `bc3_refs.introduced_in` once one exists. Nothing else in the branch depends
on the pin having moved.

Also absorbed here: creating an empty to-do list template, and the
templatification endpoints that lift an existing to-do list or card table into
the library, which bc3 documents on the same branch under
`doc/api/sections/templatifications.md`. Those carry bullets the route extractor
reads but their example markers are not yet filled from a live response, which
does not affect modelling.
