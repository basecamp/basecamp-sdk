---
gap: recording-bubble-up-write
status: partial-coverage
detected: 2026-09-02
sdk_demand: medium
smithy_refs:
  - "CreateBubbleUp operation"
  - "DeleteBubbleUp operation"
  - "CreateBubbleUpInput.at member"
bc3_refs:
  introduced_in: master
  routes:
    - POST /:account_id/recordings/:recording_id/bubble_up.json
    - DELETE /:account_id/recordings/:recording_id/bubble_up.json
    - "GET /:account_id/recordings/:recording_id/bubble_up.json (render path blocked — see below)"
  controllers:
    - app/controllers/recordings/bubble_ups_controller.rb
  related_existing_api:
    - GetBubbleUps (the /my/readings/bubble_ups.json list — see bubble-ups-surface)
    - CreateBookmark (the per-recording bookmark toggle this write surface mirrors)
---

# Recording Bubble Up write surface (create / delete, with scheduled `at`)

## What's missing

The per-recording **write** side of Bubble Up — bubbling a recording up (the
BC5 successor to "save") and popping it back down — had no SDK coverage.
[[bubble-ups-surface]] absorbed only the **read** side (the account-user
`GET /my/readings/bubble_ups.json` list and the notification counts); it did not
touch the per-recording actions. Two operations, both answering **204 No
Content** with no body from `Recordings::BubbleUpsController`:

- **`POST /recordings/{id}/bubble_up.json`** — bubbles the recording up for the
  current user. Takes an **`at`** body field controlling timing: `"now"`
  bubbles up immediately, and a scheduling keyword (`"today"`, `"tomorrow"`,
  `"weekend"`, `"next_week"`) or an ISO8601 date (e.g. `"2026-09-10"`) schedules
  it. **bc3 currently requires `at`**: `Reading::Bubbleupable#bubble_up`
  routes `"now"` to `bubble_up_now`, but any other value (including a nil from
  an omitted param) flows to `Reading::BubbleUpSchedule#bubble_up_at=`, whose
  `else` branch calls `Date.iso8601(value)` — and `Date.iso8601(nil)` raises.
  So callers must send `"now"` for the immediate case; the SDK models `at`
  optional (not `@required`) so a future bc3 `params[:at] ||= "now"` makes
  omission mean "now" with no SDK change. Idempotent: re-bubbling an
  already-bubbled recording is set-membership and still returns 204.
- **`DELETE /recordings/{id}/bubble_up.json`** — pops the current user's
  bubble-up. Idempotent: popping an absent bubble-up also returns 204.

`bubble_up` is drawn in `concern :recording_actions` (config/routes.rb:232),
included flat at the CANONICAL `resources :recordings` (:276) and separately
bucket-scoped at :918 — the same family as `resource :position` / `:spotlight`
/ `:pin`, all of which the SDK already models flat.

## The remaining gap — per-recording status GET

`GET /recordings/{id}/bubble_up.json` (the `show` action) is **not
absorbable** and stays a gap. `Recordings::BubbleUpsController#show` renders
`app/views/recordings/bubble_ups/show.json.jbuilder`, which lives **outside
`app/views/api/`**. bc3's API boundary is not route scope: `api_request?` keys
on the API host, and `restrict_view_paths_to_api_root` then limits view lookup
to `app/views/api/`. A template outside that root is unrenderable on the API
host — this is the same trap that condemned `GetRecording` (see the bc3 route
allowlist header). The `head :no_content` create/destroy actions sidestep it
(no template), which is exactly why they are absorbable and `show` is not.

The account-user list `GetBubbleUps` already covers "what has this user bubbled
up" (current + scheduled, paginated); the missing piece is the cheap
per-recording boolean `{ "bubbled_up": … }` for one recording without walking
the list.

## Why it matters

Bubble Up is the BC5 successor to the BC4 "save forever" `memories` collection.
Without the write ops a custom integration cannot bubble a recording up (nor
schedule one) or pop it back down, and there is no client-side workaround — the
relationship is private per-user and only reachable through these actions.

## Suggested API shape

- `CreateBubbleUp` → `POST /recordings/{recordingId}/bubble_up.json`, `204`,
  optional `at: String` body field, `@basecampIdempotent` (re-bubbling is
  set-membership; safe to retry).
- `DeleteBubbleUp` → `DELETE /recordings/{recordingId}/bubble_up.json`, `204`,
  no body, `@basecampIdempotent` (delete-of-absent is also `204`).
- **Status GET (blocked):** a per-recording `GetBubbleUp` →
  `GET /recordings/{recordingId}/bubble_up.json` returning
  `{ bubbled_up: Boolean }` (the `BubbleUpStatus` analogue of `BookmarkStatus`)
  **cannot** be added until BC3 moves the render path under `app/views/api/`.

## Implementation notes for BC3

- Create and destroy are shipped and API-renderable (both `head :no_content`);
  nothing pending there.
- To close the status-GET gap, add an API-root render path for the `show`
  action — an `app/views/api/recordings/bubble_ups/show.json.jbuilder`
  emitting `{ "bubbled_up": <bool> }` (mirroring how the bookmark toggle's
  `GET /recordings/{id}/bookmark.json` renders `{ "bookmarked": <bool> }` from
  `app/views/api/`) — then this entry's Smithy `GetBubbleUp` can follow.
- Documenting the create/destroy contract in `doc/api/` would also let
  `check-bc3-route-parity` drop the two `sdk_routes_absent_from_bc3_docs`
  waivers these operations currently carry.

## SDK absorption plan when this lands

**Partially absorbed.** `BubbleUpsService` models `CreateBubbleUp` (with the
optional `at`) and `DeleteBubbleUp`; the `BubbleUps` tag resolves to
`BubbleUpsService` in every generator via the default fallback (zero
overrides). Both mutations are flagged idempotent and covered by idempotency
conformance cases; the Go wrapper adds an `AccountClient.BubbleUps()` accessor
with create (at present/absent) + delete tests, and `paths.json` carries three
cases (create-with-`at`, create-without-`at`, delete) dispatched in all six runners.
The two flat routes are waived in `spec/bc3-route-allowlist.yml`
(`sdk_routes_absent_from_bc3_docs`) with routes.rb + controller evidence, since
bc3's `doc/api` documents neither spelling yet.

The per-recording **status GET** stays open here (`partial-coverage`) until BC3
provides an API-root render path, at which point a `GetBubbleUp` operation and a
`BubbleUpStatus` output shape close it.
