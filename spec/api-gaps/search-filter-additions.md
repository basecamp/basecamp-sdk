---
gap: search-filter-additions
status: partial-coverage
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

Docs shipped, params not final — **do not absorb yet**. As of the 2026-07-22
sync, `doc/api/sections/search.md` on `master` documents the filter surface:

- `type_names[]` — array of recording types to include (`key` values from
  the metadata endpoint's `recording_search_types`); the metadata endpoint
  prose now advertises `type_names[]` as the discovery target.
- `bucket_ids[]` — array of project IDs.
- `creator_ids[]` — array of creator person IDs.
- `since` — time-range filter: `last_7_days`, `last_30_days`, `last_90_days`,
  `last_12_months`, or `forever` (the default); unrecognized values
  normalize to `forever`.
- `sort` — `best_match` (default, relevance with a recency boost) or
  `recency` (strictly newest first); unrecognized values fall back to
  recency ordering.
- Deprecated-but-retained singulars for older clients: `type`, `bucket_id`,
  `creator_id` (prefer the plural array forms).

The hold stands because open BC3 **#12361** ("Search API: query-faithful
params and root cache") is still reshaping this parameter surface. The
status stays `partial-coverage` until #12361 settles: absorbing the current
param list would model a contract BC3 has already queued for change.

The `timelines/searches` route is the timeline-scoped variant; covered here
since it shares the input shape.

## Why it matters

These are additive filter params on an existing endpoint. Without them, SDK
consumers either over-fetch and filter client-side (slow, paginates wrong) or
hand-roll URL strings to bypass the typed input shape (fragile and silently
breaks if BC3 changes the param names).

## Suggested API shape

Additive parameters on the existing `SearchInput` shape, typed per whatever
`doc/api/sections/search.md` documents after #12361 merges. Response shape is
unchanged.

## Implementation notes for BC3

- All additions are query-string params handled server-side. No new
  controller actions, no new partials.
- #12361 (open) is the deciding PR for the final param names/semantics;
  `doc/api/sections/search.md` follows it.
- Defaults are now documented (`since=forever`, `sort=best_match`), along
  with the fallback behavior for unrecognized values.

## SDK absorption plan when this lands

- **Wait for BC3 #12361 to merge**, then re-derive the param list from the
  merged `doc/api/sections/search.md` and flip this entry to
  `addressed-in-bc3-pr-12361`.
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
