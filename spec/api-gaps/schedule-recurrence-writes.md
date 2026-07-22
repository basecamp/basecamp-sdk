---
gap: schedule-recurrence-writes
status: addressed-in-bc3-pr-12359
detected: 2026-07-22
sdk_demand: medium
bc3_pr: 12359
bc3_refs:
  introduced_in: five
  routes:
    - "POST /:account_id/schedules/:schedule_id/entries.json (existing — recurrence params additive)"
    - "PUT /:account_id/schedule_entries/:id.json (existing — recurrence params additive)"
  controllers:
    - app/controllers/schedules/entries_controller.rb
  related_existing_api:
    - CreateScheduleEntry
    - UpdateScheduleEntry
    - GetScheduleEntry
---

# Schedule entries — recurrence write parameters

## What's missing

Write-side recurrence for schedule entries. The read surface
(`recurrence_schedule` in Get a schedule entry) has long been documented;
BC3 **#12359** (merged 2026-07-22, post-train follow-up; cURL example
aligned by #12363) documents the **write** contract in
`doc/api/sections/schedule_entries.md` on `master`, as additive parameters
on the existing create/update endpoints:

- `recurrence_schedule` — object that makes the entry recurring:

  | Param | Applies to | Contract |
  |---|---|---|
  | `frequency` | required in the object | one of `every_day`, `every_weekday`, `every_week`, `every_other_week`, `every_month`, `every_day_of_month`, `every_year`, `custom_week`, `custom_month` |
  | `days` | `every_day`; `custom_month` without `week_instance` | for `every_day`: days of week as integers `0` (Sunday)–`6` (Saturday), omit to recur every day; for `custom_month`: day of month `1`–`31`. Derived from `starts_at` for the other frequencies |
  | `week_instance` | `every_month`, `custom_month` | which week of the month, `1`–`4`, or `-1` for the last week |
  | `week_interval` | `custom_week` | repeat every `2`–`12` weeks |
  | `month_interval` | `custom_month` | repeat every `2`–`12` months |

- `recurs_until` — top-level (sibling of `recurrence_schedule`), ISO 8601
  date the recurrence ends; omit to recur indefinitely. Reflected as
  `recurrence_schedule.end_date` in the response.
- The remaining `recurrence_schedule` attributes shown in the read payload
  (`hour`, `minute`, `start_date`, `duration`, `end_date`) are **derived**
  from `starts_at`/`ends_at`/`recurs_until` and **ignored on input**.
- An **invalid `recurrence_schedule` is silently discarded on create**: the
  entry is created without recurring (no validation error).
- Update: adding a `recurrence_schedule` makes a non-recurring entry recur.
  An entry that **already recurs can't be changed** through the update
  endpoint — it redirects to the entry's first occurrence, like Get a
  schedule entry does.

**Server-hang warning**: never send `week_instance: 0`. BC3 currently spins
computing occurrences for it (open BC3 **#12362**, "Reject recurrence
schedules whose occurrences can't be computed", adds the rejection). Until
#12362 merges, an SDK-side guard or enum constraint must make `0`
unrepresentable.

## Why it matters

Recurring events are table stakes for calendar integrations. The SDK can
read `recurrence_schedule` but can't create or update recurring entries, so
any integration syncing an external calendar into Basecamp has to fan a
recurring series out into individual entries — which then don't behave as a
series in the product.

## Suggested API shape

Additive optional members on the existing `CreateScheduleEntry` /
`UpdateScheduleEntry` inputs: a `recurrence_schedule` structure (enum-typed
`frequency`; integer list `days`; bounded integers per the table) plus
top-level `recurs_until: ISO8601Date`. Response shapes unchanged (the read
side already models `recurrence_schedule`).

## Implementation notes for BC3

- Shipped in docs — `schedules/entries_controller.rb` handles the params.
- #12362 (open) is the server-side guard for uncomputable schedules
  (`week_instance: 0` et al.); silent-discard-on-invalid on create is
  documented, not a bug.

## SDK absorption plan when this lands

- Vehicle: the §Q absorption queue's **PR-5**. Docs are merged, so the doc
  gate is cleared, but the **wire-echo probe remains a pre-modeling gate**:
  before modeling, probe a live BC5 create with each frequency family and
  diff the echoed `recurrence_schedule` against the doc (the derived-fields
  and silent-discard semantics make doc-only modeling risky).
- Model `frequency` as an enum (nine values above); constrain or guard
  `week_instance` so `0` can't be sent while BC3 #12362 is open.
- Status flips to `absorbed-in-sdk` with the absorption PR (which adds the
  Smithy refs).
- Pairwise check: BC4 accepts the same create/update routes but recurrence
  behavior on BC4 is unverified — probe both rails before asserting
  pairwise invariants.
