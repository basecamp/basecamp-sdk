---
gap: bubble-ups-surface
status: absorbed-in-sdk
detected: 2026-07-24
sdk_demand: high
bc3_pr: 11628
smithy_refs:
  - "GetMyNotificationsOutput.bubble_ups_count member (spec/basecamp.smithy:8745)"
  - "GetMyNotificationsOutput.scheduled_bubble_ups_count member (spec/basecamp.smithy:8751)"
  - "GetMyNotificationsInput.limit_bubble_ups member (spec/basecamp.smithy:8735)"
  - "GetBubbleUps operation (spec/basecamp.smithy:8803)"
bc3_refs:
  introduced_in: master
  routes:
    - "GET /:account_id/my/readings.json (existing GetMyNotifications — bubble_ups_count/scheduled_bubble_ups_count/limit_bubble_ups additive)"
    - "GET /:account_id/my/readings/bubble_ups.json (new — paginated bubble-ups list)"
  controllers:
    - app/controllers/my/readings_controller.rb
    - app/controllers/my/readings/bubble_ups_controller.rb
  related_existing_api:
    - GetMyNotifications
---

# Bubble Ups successor surface (counts, limit param, dedicated list)

## What's missing

The bubble-up successor surface on `doc/api/sections/my_notifications.md`
(codified by BC3 **#11628**) that
[[memories-emptied-regression]] explicitly **defers to a separate additive
PR**. That entry settles the subtractive `memories: []` delta and — by its own
scope statement — leaves the adjacent bubble-up additions to this entry:

- **`bubble_ups_count` / `scheduled_bubble_ups_count`** — top-level integer
  counts on `GET /my/readings.json`, for notification-UI badges. Present
  independent of the `limit_bubble_ups` cap (and `scheduled_bubble_ups_count`
  is returned even when the `scheduled_bubble_ups` array is omitted).
- **`limit_bubble_ups`** — optional boolean query param on `GET
  /my/readings.json`: set `true` to cap `bubble_ups` at 2 current items and
  **omit the `scheduled_bubble_ups` key entirely**. Defaults to `false`.
- **`GET /my/readings/bubble_ups.json`** — a **new** dedicated operation
  returning the current user's current and scheduled bubble-ups as a
  **paginated bare array** (50 per page, Link-header pagination). Current
  bubble-ups are returned first (ordered by most recently bubbled up), then
  scheduled bubble-ups (ordered by scheduled bubble-up time). Each item uses
  the same notification object shape as `GET /my/readings.json`. This is **not**
  "the ≤50 most-recently-read `bubble_ups` field" — it is the full,
  page-through-able current+scheduled list.

The grouped `bubble_ups` / `scheduled_bubble_ups` arrays and their notification
item fields (`bubble_up_url`, `bubble_up_at`, …) were **already modeled** on
`GetMyNotificationsOutput` / `Notification`; this entry does not re-add them.

## Why it matters

Bubble Up is the BC5 successor to the BC4 "save forever" `memories` collection.
Without the counts, integrations can't render notification badges without
over-fetching; without the dedicated endpoint, they can only see the capped
`bubble_ups` field on the readings payload and cannot page through the full
current+scheduled set; without `limit_bubble_ups`, a lightweight badge fetch
still pays for the full scheduled list.

## Suggested API shape

- Additive required `bubble_ups_count` / `scheduled_bubble_ups_count` integers
  on the existing `GetMyNotificationsOutput`.
- Additive optional `limit_bubble_ups: Boolean` `@httpQuery` on
  `GetMyNotificationsInput`.
- New `GetBubbleUps` operation (`GET /my/readings/bubble_ups.json`) with an
  optional `page` query param and `@basecampPagination(style: "link", …)`,
  returning a bare `NotificationList`.

## Implementation notes for BC3

Shipped — nothing pending. `my/readings_controller.rb` renders the counts and
honors `limit_bubble_ups`; `my/readings/bubble_ups_controller.rb` serves the
paginated dedicated list. `doc/api/sections/my_notifications.md` on `master`
is the contract of record (codified by BC3 #11628).

## SDK absorption plan when this lands

Absorbed (basecamp-sdk PR-3 of the post-#401 follow-up program).

- Added required `bubble_ups_count` / `scheduled_bubble_ups_count` to
  `GetMyNotificationsOutput` and optional `limit_bubble_ups` to
  `GetMyNotificationsInput`.
- Added the new `GetBubbleUps` operation (bare-array, Link-paginated, `page`
  query param) with tag/service mapping and the Go wrapper
  (`MyNotificationsService.BubbleUps`, following the Link header across all
  pages by default; a positive page returns only that page).
- The Go `Get` wrapper gained a non-breaking `WithLimitBubbleUps()` option.
- Tests: a Go **multi-page** pagination test for `BubbleUps` (Link-following +
  `X-Total-Count` metadata, current-then-scheduled ordering), a single-page
  test, and a test proving `limit_bubble_ups=true` sends the param and decodes
  a response that **omits `scheduled_bubble_ups`** while keeping the counts.
- Registry: this is a **new** entry — [[memories-emptied-regression]] keeps its
  status and `pairwise`-retirement explanation intact; its scope was the
  subtractive empty-`memories` delta, which explicitly deferred these
  successors here.
- Canary: a `GetBubbleUps` entry validates statically in
  `live-my-surface.json` (the live canary is dormant pending a safe token
  mechanism).
