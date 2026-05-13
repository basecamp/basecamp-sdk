---
gap: search-filter-additions
status: no-json-contract
detected: 2026-05-01
sdk_demand: medium
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3e
  routes:
    - "GET /:account_id/search.json (existing)"
    - "GET /:account_id/timelines/searches.json (existing — covered here)"
  controllers:
    - app/controllers/searches_controller.rb
  related_existing_api:
    - SearchService.search
---

# Search — additional filter parameters

## What's missing

The existing Search endpoint accepts a small set of filter parameters today.
BC5 adds the following (per BC3 plan Phase 3e):

- `type_names` (string[]) — filter to specific recording types.
- `creator_ids` (long[]) — filter to recordings authored by specific people.
- `bucket_ids` (long[]) — restrict to specific projects.
- `exclude_chat` (boolean) — drop chat messages from results.
- `file_type` (string) — filter file recordings by extension/type.
- `sort` (enum: "relevance" | "recency" | …) — control result ordering.

The `timelines/searches` route is the timeline-scoped variant; covered here
since it shares the input shape.

## Why it matters

These are additive filter params on an existing endpoint. Without them, SDK
consumers either over-fetch and filter client-side (slow, paginates wrong) or
hand-roll URL strings to bypass the typed input shape (fragile and silently
breaks if BC3 changes the param names).

## Suggested API shape

Additive parameters on the existing `SearchInput` shape — types per the list
above. Response shape is unchanged.

## Implementation notes for BC3

- All additions are query-string params handled server-side. No new
  controller actions, no new partials.
- `doc/api/sections/search.md` updates the parameter list.
- Document defaults explicitly (e.g. `sort: "relevance"`).

## SDK absorption plan when this lands

- Extend the existing Smithy `SearchInput` structure with the new optional
  fields (each annotated `@httpQuery`).
- Same change applies to the timeline-search input if it's a separate
  Smithy structure.
- No new service registrations.
- Canary: add a search call with at least one of the new filter params in
  `live-my-surface.json`.
- Pairwise check: existing `search.json` is BC4-compatible; new params are
  silently ignored on BC4, present and respected on BC5. No invariant
  violation.
