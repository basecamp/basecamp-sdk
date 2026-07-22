---
gap: calendar
status: addressed-in-bc3-pr-12321
detected: 2026-05-01
sdk_demand: medium
bc3_pr: 12321
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

- New Smithy operations: `GetCalendar`, `UpdateCalendar`, with shapes derived
  from the merged doc's examples.
- New service registration: `CalendarsService` with `get(id)` and
  `update(id, input)`.
- Status flips to `absorbed-in-sdk` with the absorption PR (which adds the
  Smithy refs).
- Canary fixture: add `GetCalendar` to `live-my-surface.json` once a stable
  fixture-id resolution path exists (e.g., dock walk → calendar tool).
- Pairwise check: BC4 absent → BC5 present is fine; the assertion is structural
  conformance against the response schema, not pairwise equality.
