---
gap: scratchpad
status: no-json-contract
detected: 2026-05-01
sdk_demand: medium
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3b
  routes:
    - "GET /:account_id/my/navigation/notes.json (URL pending — BC3 plan defers final path)"
    - "PUT /:account_id/my/navigation/notes.json (URL pending)"
  controllers:
    - app/controllers/my/notes_controller.rb (path tentative — actual controller name pending BC3 decision)
  related_existing_api: []
---

# Scratchpad (per-user notes)

## What's missing

BC5 introduces a per-user "scratchpad" — a single rich-text note attached to
the navigation surface, distinct from the per-project Documents resource.
Read/write is exposed in the web UI but no JSON contract exists yet.

The BC3 parity plan Phase 3b ships the jbuilder partial and a JSON branch on
the relevant controller, but **the URL path is explicitly deferred** by the
BC3 team. The plan's working default is `/my/navigation/notes.json`; the
final path is BC3's call. This brief tracks the decision.

## Why it matters

External SDK consumers building dashboards or note-syncing tools can't read or
write the user's scratchpad without screen-scraping. As the navigation gets
more first-class data attached to it, leaving notes off the API surface is a
predictable source of demand-signal complaints.

## Suggested API shape

`GET /:account_id/my/navigation/notes.json` (or final-decided path):
- `id` (long, optional — single per-user note may not need an id)
- `content` (rich text, format consistent with Document/Message bodies)
- `updated_at`, `created_at`

`PUT /:account_id/my/navigation/notes.json`:
- `content` (string)
- Returns 204 or the updated payload.

## Implementation notes for BC3

- Resolve URL path first. Coordinate with the BC3 plan owner; once the path
  is fixed, update this brief's `routes:` field and the corresponding SDK
  Smithy operation paths.
- Single-resource (not collection): the `index/show` distinction collapses
  here — there's one note per user.
- Add to `doc/api/`.

## SDK absorption plan when this lands

- Smithy: `GetMyScratchpad`, `UpdateMyScratchpad` (or `MyNotesService` —
  service name follows whatever BC3 picks for the URL).
- Service registration depends on URL: if `/my/notes.json`, group with the
  `My*` services; if under `/my/navigation/...`, follow that namespacing.
- Canary fixture: a single per-account fixture-id-free GET works well since
  the resource is implicit-self.
- Pairwise check: BC4 absent → BC5 present is fine.

**Action item**: revisit this brief when BC3 commits the URL path.
