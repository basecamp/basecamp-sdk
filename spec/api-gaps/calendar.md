---
gap: calendar
status: absorbed-in-sdk
detected: 2026-05-01
sdk_demand: medium
bc3_pr: 12321
smithy_refs:
  - "GetCalendar operation"
  - "UpdateCalendar operation"
  - "Calendar structure"
  - "CalendarAttributes structure"
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3b
  routes:
    - GET /:account_id/calendars/:id.json
    - PUT /:account_id/calendars/:id.json
  controllers:
    - app/controllers/calendars_controller.rb
  related_existing_api: []
---

# Calendar (show/update)

## What's missing

SDK absorption only — the contract shipped via BC3 **#12321** in the BC5 API
train (2026-07-18..21). `doc/api/sections/calendars.md` on `master` is the
contract of record, documenting the planned **show + update only** scope
(not full CRUD):

- `GET /calendars/:id.json` — returns the calendar (keyed by bucket id).
- `PUT /calendars/:id.json` — updates the calendar.

The Calendar is a top-level BC5 resource (a calendar view distinct from the
per-project Schedule) with no BC4 analog, so additive coverage is safe.

## Why it matters

Without `GET` an SDK client can't display the user's calendar surface in a
custom integration. Without `PUT` consumers can't set the mutable properties
the web UI exposes. This is a new top-level resource on BC5 with no BC4
analog, so additive coverage is safe.

## Suggested API shape

Per the merged `doc/api/sections/calendars.md` — derive the exact field list
from the doc's captured examples at absorption time rather than restating it
here (the doc examples are regenerated from live BC5 by the #11629 tooling).

## Implementation notes for BC3

Shipped — nothing pending. `calendars_controller.rb` serves both routes with
JSON branches; `doc/api/sections/calendars.md` documents them. Re-evaluate
`index/create/destroy` only if usage signals demand later — the shipped scope
is intentionally small.

## SDK absorption plan when this lands

**Absorbed** (post-#504 program C8): `CalendarsService` models the show +
update pair with the shape taken from `doc/api/sections/calendars.md` **at
`e83b2733`, the provenance pin when this entry was verified** (no doc or
controller drift up to it): id, type, name, color, created_at, updated_at, url,
app_url, schedule_url — all required non-nullable. The update body is the
nested `{calendar: {color}}` envelope; that revision's controller returns the
422 contract (`{"errors": {"color": ["is not a valid color"]}}`, rejected up
front for unknown enum values), documented on the operation and pinned in
per-SDK 422 tests. `UpdateCalendar` is a flagged idempotent PUT with an
idempotency conformance case. The `GetCalendar`
live-canary case ships in `live-my-surface.json` with an **env-var-only**
fixture (`BASECAMP_BC5_CALENDAR_ID` → `BASECAMP_CALENDAR_ID` → skip): no
modeled endpoint returns a calendar bucket id (the dock has no calendar tool;
bucket `type` never says "Calendar"), so SDK discovery is impossible until
upstream exposes one — revisit the fixture ladder's discovery branch if that
surface ever ships.
