---
gap: search-filter-additions
status: absorbed-in-sdk
detected: 2026-05-01
sdk_demand: medium
bc3_pr: 12361
smithy_refs:
  - SearchInput
  - SearchMetadata
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3e
  routes:
    - "GET /:account_id/search.json (existing)"
    - "GET /:account_id/searches/metadata.json (existing)"
  controllers:
    - app/controllers/concerns/searching.rb
    - app/models/search.rb
  related_existing_api:
    - Search
    - GetSearchMetadata
---

# Search — additional filter parameters

## What's missing

**Absorbed** (SDK #399). BC3 **#12361** ("Search API: query-faithful params
and root cache") merged `f0d9387a58` (2026-07-22), settling the parameter
surface; the merged `doc/api/sections/search.md` (cross-checked against
`app/controllers/concerns/searching.rb` + `app/models/search.rb`) documents:

- `type_names[]` — array of recording types to include (`key` values from
  the metadata endpoint's `recording_search_types`).
- `bucket_ids[]` — array of project IDs.
- `creator_ids[]` — array of creator person IDs.
- `file_type` — attachment type filter (`key` from `file_search_types`).
- `exclude_chat` — boolean; excludes chat results.
- `since` — time-range filter: `last_7_days`, `last_30_days`, `last_90_days`,
  `last_12_months`, or `forever` (the default); unrecognized values
  normalize to `forever`.
- `sort` — `best_match` (default, relevance with a recency boost) or
  `recency` (strictly newest first); unrecognized values fall back to
  recency ordering.
- Deprecated-but-retained singulars for older clients: `type`, `bucket_id`,
  `creator_id` (prefer the plural array forms).

**Wire-format note (critical):** the merged controller permits the arrays via
`permit(... type_names: [], creator_ids: [], bucket_ids: [])`. Rack only parses
the **bracketed repeated** form (`bucket_ids[]=1&bucket_ids[]=2`) into an
array; comma-joined or bare-repeated forms are dropped. The absorption models
this with bracketed `@httpQuery("bucket_ids[]")` wire names; the owned
generators strip the `[]` from the public identifier and emit the bracketed
key on the wire (Ruby/Faraday re-adds `[]`, so it emits the stripped key).

Also absorbed: the **metadata** endpoint's real shape. The prior model's
fictional `SearchMetadata { projects }` is replaced with
`recording_search_types[]` / `file_search_types[]` (each `{key, value}`, `key`
nullable) and the five `default_*_label` fields returned by
`GET /searches/metadata.json`.

**Route correction:** an earlier version of this entry listed a
`GET /:account_id/timelines/searches.json` "timeline-scoped variant". **That
route does not exist** in bc3 (verified against `config/routes.rb`: only
`/search.json`, `/searches/metadata.json`, and legacy `/search/files|pings`).
The claim was factually wrong and has been removed; there is no separate
timeline-search input shape to model.

## Why it matters

These are additive filter params on an existing endpoint. Without them, SDK
consumers either over-fetch and filter client-side (slow, paginates wrong) or
hand-roll URL strings to bypass the typed input shape (fragile and silently
breaks if BC3 changes the param names).

## Suggested API shape

Additive query parameters on the existing `SearchInput` shape (bracketed
`@httpQuery` wire names for the arrays), plus the corrected `SearchMetadata`
shape. Response shape of `GET /search.json` is unchanged.

## Implementation notes for BC3

- All additions are query-string params handled server-side. No new
  controller actions, no new partials.
- #12361 is the settling PR for the final param names/semantics;
  `doc/api/sections/search.md` follows it.
- Defaults are documented (`since=forever`, `sort=best_match`), along with the
  fallback behavior for unrecognized values.

## SDK absorption plan when this lands

Done in SDK #399:

- Extended `SearchInput` with `typeNames`/`bucketIds`/`creatorIds` (bracketed
  `@httpQuery` wire names), `fileType`/`excludeChat`/`since`, and the
  `@deprecated` singulars `type`/`bucketId`/`creatorId`.
- Updated `SearchSortField` doc (`best_match|recency`) and added
  `SearchSinceField`; this narrows only the generated **TS** `sort` union
  (`created_at`→`recency`) — all other SDKs expose plain string.
- Replaced fictional `SearchMetadata { projects }` (+ `SearchProject`) with
  the real `recording_search_types`/`file_search_types` (`SearchType {key,
  value}`) and five `default_*_label` fields. `SearchType.key` is
  **required-and-nullable** — BC3's jbuilder always emits `key`, with `null`
  for the default "everything"/"all files" option. Smithy has no native
  required-nullable, so the OpenAPI models it via `smithy-build.json` `jsonAdd`:
  `type: ["string", "null"]` plus `key` added to `required`. The typed surfaces
  therefore mark key present-but-nullable rather than optional or null-lossy —
  Go `*string` (via `x-go-type`, `json:"key"` no omitempty), TS `string | null`,
  Python `str | None`, Swift `let key: String?`. The Swift model generator
  keeps requiredness and nullability separate: nullable sets the optional value
  type, required sets presence — a required-nullable member gets explicit
  Codable (`decode(String?.self)` rejects a missing key but accepts null;
  `encode` round-trips nil back to `"key": null`).
- Taught the six owned generators the bracketed-array rule (public identifier
  strips `[]`; wire key stays bracketed, except Ruby which emits the stripped
  key because Faraday re-adds `[]`). Go maps the fields onto the generated
  client, which owns the wire (`form:"bucket_ids[]"`) — no URL rewriting. The
  array params are generated as *pointer* slices (`*[]int64`, via an
  `enhance-openapi` pass) so an unset filter is omitted entirely; a non-pointer
  optional slice would serialize an empty `bucket_ids[]=` that Rails normalizes
  to a bogus `[0]` filter. Ruby has the same empty-array hazard: its query
  builder uses `compact_query_params` (drops empty arrays, not just nils — body
  params keep `compact_params`) so an empty `bucket_ids: []` filter is omitted
  rather than encoded as a bare `bucket_ids[]`. Ruby's `SearchType#to_h` keeps a
  required-nullable `key` present (not `.compact`-ed away) so the default
  option's explicit null survives.
- Per-SDK wire tests assert the decoded query carries `bucket_ids[]` with the
  right values and no bare/double-bracketed keys, plus a full-surface test that
  exercises every param (arrays, `file_type`/`exclude_chat`/`since`/`sort`, and
  the deprecated singulars). Go covers both the generated request and the
  public wrapper (which also exposes the deprecated `Type`/`BucketID`/
  `CreatorID`). Metadata-decoding coverage (incl. the null `key`) across all
  six SDKs; Python covers sync + async.
- Canary: added a `Search` entry driving `type_names[]` + `since` in
  `live-my-surface.json` (acceptance/decoding coverage — see the honesty note
  there; the live vocabulary can't assert the backend respects the filter).
- Pairwise check: existing `search.json` is BC4-compatible; new params are
  silently ignored on BC4, accepted on BC5. No invariant violation.
