---
gap: calendar
status: no-json-contract
detected: 2026-05-01
sdk_demand: medium
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

BC5 introduces a top-level Calendar resource (a per-user calendar view, not the
per-project Schedule). The web app reads and updates calendar settings via
`calendars_controller.rb`, but there are no JSON-formatted responses on those
routes today. The BC3 parity plan Phase 3b ships **show + update only** —
not full CRUD.

## Why it matters

Without `GET` an SDK client can't display the user's calendar surface in a
custom integration. Without `PUT` consumers can't set the few mutable
properties (e.g. visibility, default view) the web UI exposes. This is a new
top-level resource on BC5 with no BC4 analog, so additive coverage is safe.

## Suggested API shape

`GET /:account_id/calendars/:id.json`:
- `id` (long), `name` (string), `url`, `app_url`
- `view` ("day" | "week" | "month" | ...) and any other settings the form exposes
- `creator` (Person), `created_at`, `updated_at`
- `subscribed_buckets` (array of Bucket-shaped) if applicable

`PUT /:account_id/calendars/:id.json`:
- Accept the mutable subset of the show payload
- Return `204 No Content` on success or the updated resource

## Implementation notes for BC3

- New `app/views/api/calendars/_calendar.json.jbuilder` partial; `show.json.jbuilder`,
  `update.json.jbuilder` reuse it.
- Add `respond_to :json` (or per-action `format.json`) to the relevant actions
  in `calendars_controller.rb`.
- Add `doc/api/sections/calendars.md` describing both routes.
- Re-evaluate `index/create/destroy` if usage signals demand later — initial
  scope is intentionally small.

## SDK absorption plan when this lands

- New Smithy operations: `GetCalendar`, `UpdateCalendar`.
- New shapes: `Calendar`, `UpdateCalendarInput`/`Output`.
- New service registration: `CalendarsService` with `get(id)` and `update(id, input)`.
- Canary fixture: add `GetCalendar` to `live-my-surface.json` once a stable
  fixture-id resolution path exists (e.g., dock walk → calendar tool).
- Pairwise check: BC4 absent → BC5 present is fine; the assertion is structural
  conformance against the response schema, not pairwise equality.
