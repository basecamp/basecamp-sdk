---
gap: notifications-sort-pings-first
status: partial-coverage
detected: 2026-08-05
sdk_demand: low
bc3_pr: 12396
bc3_refs:
  introduced_in: pings-sort-preference (BC3 #12396, merged 98eb24b22f)
  routes:
    - GET /:account_id/my/notifications.json
    - PUT /:account_id/my/notifications.json
  controllers:
    - app/controllers/my/notifications_controller.rb
  views:
    - app/views/api/my/notifications/show.json.jbuilder
  related_existing_api:
    - GetMyNotifications (a DIFFERENT endpoint — /my/readings.json, the feed)
    - UpdateQuestionNotificationSettings (the only notification-settings operation modelled, unrelated)
---

# The notification settings resource, and its new `sort_pings_first` field

## What's missing

**`GetMyNotifications` is not this endpoint.** Read that first, because the name
collision is the whole trap here. The SDK's `GetMyNotifications` points at
**`/{accountId}/my/readings.json`** (`spec/basecamp.smithy`) — the notification
*feed*, a list of things that happened. The endpoint BC3 #12396 changed is the
separate notification *settings* resource, **`/{accountId}/my/notifications.json`**,
which is a single object describing how the user wants to be notified. The SDK
models **nothing** for it in either direction. The only notification-settings
operation in the spec is `UpdateQuestionNotificationSettings`, which is about a
single question's check-in subscribers and is unrelated. Anyone matching on the
operation name will conclude this is already covered; it is not.

The settings resource has existed for a while and was never modelled. What
#12396 adds is a new field on it and a way to write that field:

- **`sort_pings_first`** (boolean) on the payload, alongside the existing
  `state`, `hide_badge_counts`, and the conditional `refresh_in` / `refresh_at`
  pair — `app/views/api/my/notifications/show.json.jbuilder`.
- **`My::NotificationsController#update`**, which persists exactly that one
  field: `@notifications_setting.update! sort_pings_first:
  params.require(:sort_pings_first)`, then renders `show`. `require` means the
  key is mandatory, not optional.

The controller also already had `create` (turn notifications on) and `destroy`
(turn them off), both rendering the same `show` payload for JSON. None of the
four actions is modelled.

bc3 documents **none** of this — there is no `doc/api/sections` entry for the
notification settings resource, so no row appears in `spec/bc3-routes.json` and
the route-parity gate sees nothing. But it renders under `app/views/api`, which
is the second half of bc3's API-ness test (drawn *and* renderable under
`restrict_view_paths_to_api_root`), so it is real API surface. That combination —
renderable but undocumented — is exactly `partial-coverage`.

## Why it matters

Low demand, recorded for correctness rather than urgency. The reason it is worth
a brief at all is the name collision: `GetMyNotifications` reads like coverage of
`/my/notifications.json` and is not, so a future reader triaging notification
work can easily mis-scope it. Writing that down is the point.

The field itself is a genuine user preference — whether pings sort above other
notifications — and an integration that manages a person's notification setup
(onboarding automation, a preferences-sync tool) cannot read or set it, or read
the notification on/off state, through the SDK today.

## Suggested API shape

bc3 should document the resource before the SDK models it; the shape below is
what the controller and view already do, not a proposal for new behaviour.

- `GET /{accountId}/my/notifications.json` → 200, an object with `state`,
  `hide_badge_counts`, `sort_pings_first`, and `refresh_in` / `refresh_at`
  present only while notifications are snoozed.
- `PUT /{accountId}/my/notifications.json` with `{"sort_pings_first": true}` →
  200 and the full settings object. The key is required.
- `POST` (on) and `DELETE` (off) both return the same object for JSON callers.

Note the conditional pair: `refresh_in` and `refresh_at` are emitted only when
`off_until` is set, so both must be modelled optional. `hide_badge_counts` is
the *negation* of the underlying `show_badge_counts?` — model the wire spelling,
not the column.

## Implementation notes for BC3

- Document the resource in `doc/api/sections/`, most naturally a new section or
  an addition to the existing "my" surface, covering all four verbs and the
  conditional snooze fields. Without a `doc/api` bullet the route stays invisible
  to `spec/bc3-routes.json` and so to the SDK's parity gate.
- Consider whether `update` should accept a sparse body. `params.require` makes
  `sort_pings_first` mandatory, so a caller cannot PUT the resource to change
  nothing, and a future second writable field would need a decision about
  omission semantics rather than inheriting one.
- The migration history is worth noting for anyone reading the range: the column
  arrived as `sort_pings_and_mentions_first` (`9cf4e17817`), was split
  (`09ff95f35d`), renamed (`0e015cacd6`), then re-landed as `sort_pings_first`
  outright (`8c5af3956f`) with the default flipped after the rename
  (`2480131f78`). Only the final name is on the wire.

## SDK absorption plan when this lands

- Model the settings resource as its own operations — `GetMyNotificationSettings`
  and `UpdateMyNotificationSettings` are the names that do not collide with
  `GetMyNotifications`. Do **not** extend `GetMyNotifications`; it is a different
  path returning a different kind of thing.
- Tag them in `spec/overlays/tags.smithy`. `MyNotificationsService` already
  exists as a generated service for the readings family, so the tag can reuse it
  — but say so deliberately in review, because grouping settings with the feed is
  the same conflation this brief exists to prevent.
- `hide_badge_counts` and the optional `refresh_in`/`refresh_at` need fixtures
  covering both the snoozed and not-snoozed shapes; a fixture that only ever
  shows the snoozed form would make two optional fields look required.
- Update the counts in `SPEC.md`, `SECURITY.md` and
  `scripts/check-idempotency-parity` — a PUT is naturally idempotent and lands in
  both the idempotent and union counts.

## Deferred (2026-08-06): absorb only after a `doc/api` PR to bc3

Considered for absorption alongside the authorization-document brief and
deliberately deferred, so the analysis is not re-derived from scratch next time.

bc3 documents this resource **nowhere**: zero hits for `my/notifications` and
zero for `sort_pings_first` across all of `doc/api/`. The trap is that
`doc/api/sections/my_notifications.md` **exists** and is entirely about the
*feed* (`/my/readings.json`) — it collides with the controller name exactly as
`GetMyNotifications` collides with the settings resource, which is the
conflation this brief already warns about, arriving one level up in the
evidence rather than in the modelling.

The consequence is mechanical: SDK operations here would be routes the SDK draws
and `doc/api` does not, so they trip route-parity **direction 1** twice and need
`sdk_routes_absent_from_bc3_docs` waivers. That list enforces almost nothing —
its `reason:` is review-only — so the waiver is a claim a reviewer accepts, not
one a gate checks. Both existing entries at least have a documented sibling
resource, making "the docs are merely incomplete here" checkable against
something. This resource has **no documented sibling at all**, so the same
sentence would be materially weaker, and the gate would not know the difference.

Absorbing first and waiving would therefore spend the waiver list's credibility
to skip the cheaper fix. **The right first move is a `doc/api` PR to bc3** — the
first bullet of "Implementation notes for BC3" above — after which the route
becomes visible to `spec/bc3-routes.json` and the SDK side needs no waiver at
all. Status stays `partial-coverage`; nothing else changes.
