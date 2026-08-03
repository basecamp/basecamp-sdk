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
- An **invalid `recurrence_schedule` is silently discarded on create** — the
  entry is created without recurring and the response simply omits
  `recurrence_schedule` (no validation error, still `201`). This is **one
  uniform rule with no error branch**: it covers uncomputable schedules
  (`week_instance: 0` and the like) exactly as it covers every other invalid
  shape. See the correction below — an earlier revision of this entry claimed
  #12362 carved uncomputable schedules out into a wire-visible validation
  error, and that is wrong.
- Update: adding a `recurrence_schedule` makes a non-recurring entry recur.
  An entry that **already recurs can't be changed** through the update
  endpoint — it redirects to the entry's first occurrence, like Get a
  schedule entry does.

**Correction — what BC3 #12362 actually changed.** #12362 (`ec17b83c`,
"Reject recurrence schedules whose occurrences can't be computed", merged
2026-07-22) tightened `RecurrenceSchedule`'s *model validations*: `week_instance`
inclusion went from `(-1..4)` to `[ -1, 1, 2, 3, 4 ]`, a new
`days_must_fit_frequency` validation was added, and `days=` stopped silently
dropping uncoercible members. It changed **no controller and no view** — its
diff touches `app/models/recurrence.rb`, `app/models/recurrence_schedule.rb`,
one HTML field partial, and model tests.

The wire consequence follows from
`Schedule::Entry::Recurrable#ensure_valid_recurrence_schedule`, a `before_save`
that predates #12362 and was untouched by it:

```ruby
unless recurrence_schedule.valid? && has_many_occurrences?
  self.recurrence_schedule = self.recurs_until = self.time_zone_name = nil
end
```

Ruby short-circuits `&&`, so making `valid?` false is precisely what stops
`has_many_occurrences?` from being reached — that is the hang fix. The record
then saves with the recurrence nilled out. So `week_instance: 0` is **silently
discarded, not rejected**: the caller gets `201` with a non-recurring entry, the
same outcome as any other invalid shape. Nothing on the wire distinguishes the
two, which is why there is no error branch to model.

An SDK-side guard remains **not mandatory** (there is no hang to avoid), but the
reason stated in the prior revision — "`0` is a normal validation error on the
wire" — was never true. Modeling `frequency` as an enum and bounding the
recurrence integers is still worthwhile, now for a different reason: silent
discard means a client that sends a bad value gets a plausible-looking success
and a silently non-recurring entry. Client-side bounds are the only thing that
turns that into a diagnosable error.

**Also silently discarded: patterns yielding fewer than two occurrences.**
`has_many_occurrences?` is `schedule_with_time_zone.first(2).size == 2`
(`app/models/schedule/entry/occurrences.rb`). A perfectly valid pattern whose
`recurs_until` leaves room for only one occurrence takes the same discard path.
This is documented nowhere in `doc/api` and is a modelling-relevant behavior:
"valid per the parameter table" is not sufficient for the entry to come back
recurring.

## Why it matters

Recurring events are table stakes for calendar integrations. The SDK can
neither create nor update recurring entries — and, as the correction under
"Suggested API shape" records, it cannot read the recurrence either: nothing in
`spec/basecamp.smithy` models `recurrence_schedule`. So any integration syncing
an external calendar into Basecamp has to fan a recurring series out into
individual entries — which then don't behave as a series in the product — and
cannot even tell that an entry it reads back is part of one.

## Suggested API shape

Additive optional members on the existing `CreateScheduleEntry` /
`UpdateScheduleEntry` inputs: a `recurrence_schedule` structure (enum-typed
`frequency`; integer list `days`; bounded integers per the table) plus
top-level `recurs_until: ISO8601Date`.

**Correction — the response side is not already modeled.** A prior revision of
this entry said "Response shapes unchanged (the read side already models
`recurrence_schedule`)". It does not. There is **no occurrence of `recurrence`
in `spec/basecamp.smithy` or `openapi.json`** other than four doc-comment
mentions of *recurring* entries redirecting, and `ScheduleEntry` carries no
`recurrence_schedule` member. Absorption therefore has to add the output
structure as well as the input one — that is scope, not a blocker, but the plan
must budget for it.

The output structure is fully determined by
`RecurrenceSchedule#as_json` → `attributes` → `pattern_attributes` +
`window_attributes`, which is a fixed **ten**-key object emitted on every
recurring entry:

| Key | Source | Note |
|---|---|---|
| `frequency` | submitted | the nine-value enum; always populated |
| `days` | submitted or derived | sorted integer list; see the derivation below. May be `null` for `every_day`/`every_weekday`, which have canonical defaults |
| `hour`, `minute` | derived | always populated: from `starts_at` in the entry's time zone, or `0`/`0` when `all_day` |
| `week_instance` | submitted | reader returns `null` unless `frequency` is `every_month`/`custom_month` |
| `week_interval` | submitted | reader returns `null` unless `frequency` is `custom_week` |
| `month_interval` | submitted | reader returns `null` unless `frequency` is `custom_month` |
| `start_date` | derived | `starts_at` as a date |
| `duration` | derived | integer **seconds**, present only when `ends_at` was supplied |
| `end_date` | derived | `recurs_until`, or `null` |

All ten keys are always **present**; most are nullable. `frequency`, `hour`,
`minute`, and `start_date` are always populated on a recurring entry; the other
six must be modeled nullable, because the frequency-gated readers return `nil`
for the inapplicable frequencies rather than omitting the key and `as_json`
emits the full hash regardless. **`doc/api` under-describes this** — the
illustrative `recurrence_schedule` object in `schedule_entries.md` shows seven
keys, omitting `week_interval`, `month_interval`, and `duration`. That block sits
*outside* the `<!-- START ... -->` / `<!-- END ... -->` markers that delimit the
#11629 live-regenerated snapshots, so it is hand-authored prose, not a captured
wire echo. Model from the source cited below, not from that example.

## Implementation notes for BC3

- Shipped in docs — `schedules/entries_controller.rb` handles the params.
- #12362 (`ec17b83c`, merged) is the server-side guard for uncomputable
  schedules (`week_instance: 0` et al.). It stops the create-time hang by
  making them fail `RecurrenceSchedule#valid?`; the resulting wire behavior is
  the pre-existing silent discard, **not** a validation error. See the
  correction under "What's missing".
- Silent-discard-on-invalid on create is documented, not a bug — but the
  sub-two-occurrences case is undocumented. `doc/api/sections/schedule_entries.md`
  is worth a follow-up sentence there.

### Source citations at the pinned revision

Everything the absorption needs is readable offline at the `bc3` revision in
[`spec/api-provenance.json`](../api-provenance.json). These are the four
load-bearing files:

| Claim | File | Mechanism |
|---|---|---|
| Which keys are accepted on input | `app/controllers/schedules/entries/base_controller.rb` | `params.require(:schedule_entry).permit(..., :recurs_until, recurrence_schedule: [ :frequency, :week_instance, :week_interval, :month_interval, days: [] ])` |
| `hour`/`minute`/`start_date`/`duration`/`end_date` ignored on input | same permit list | they are **absent from it** — strong parameters filters them before the model is ever constructed |
| those five derived on output | `app/models/schedule/entry/recurrable.rb` | `sync_recurrence_schedule_dates`, a `before_validation` that assigns `start_date`/`hour`/`minute` from `starts_at`, `duration` from `ends_at`, `end_date` from `recurs_until` |
| `days` overwritten for most frequencies | same method | assigned `start_date.wday` for `every_week`/`every_other_week`/`custom_week`/`every_month` and for `custom_month` **with** a `week_instance`; `start_date.day` for `every_day_of_month`. A submitted `days` survives only for `every_day`, `every_weekday`, `every_year`, and `custom_month` **without** a `week_instance` |
| silent discard on invalid | same file | `ensure_valid_recurrence_schedule`, a `before_save` gated on `new_record? && recurring?` |
| the response key set | `app/models/recurrence_schedule.rb` | `as_json` → `attributes` → `pattern_attributes` + `window_attributes` (ten keys) |
| the validation bounds | same file | `frequency` inclusion (nine values), `days` length `1..7` unless yearly, `days_must_fit_frequency`, `hour` `0..23`, `minute` `0..59`, `week_instance` `[-1,1,2,3,4]`, `week_interval` / `month_interval` `2..12` |
| the echo is `as_json` verbatim | `app/views/api/schedules/entries/_entry.json.jbuilder` | `json.recurrence_schedule recording.recordable.recurrence_schedule`, emitted only `if recording.recurring?` |

## SDK absorption plan when this lands

- Vehicle: the §Q absorption queue's **PR-5**. Docs are merged, so the doc gate
  is cleared.
- Model `frequency` as an enum (nine values above), model the ten-key response
  structure per the table under "Suggested API shape", and bound the recurrence
  integers client-side — not to avoid a hang (#12362 removed that) but because
  the server's failure mode is a silent discard that a caller cannot otherwise
  detect.
- Status flips to `absorbed-in-sdk` with the absorption PR (which adds the
  Smithy refs).

### Pre-modeling gate: reassessed 2026-08-03

A prior revision made a **live wire-echo probe a hard pre-modeling gate**, on
two rails: probe BC5, and probe BC4 "before asserting pairwise invariants".
Both halves are re-decided here.

**The BC4 half is unsatisfiable and is withdrawn — not deferred, withdrawn.**
BC5 replaced BC4 in production; there is no live BC4 backend to compare
against. That is recorded in
[`COORDINATION.md`](../../COORDINATION.md) twice — contract decision 1
("BC5 replaced BC4 in production — there is no live BC4 backend to compare
against, so the pairwise machinery was removed") and contract decision 3
("settled by release: BC5 replaced BC4 in production, so there is no live BC4
backend to shim") — and again in the `conformance-canary` recipe in the
`Makefile` ("BC5 replaced BC4 in production, so there is one live backend …
The former pairwise BC4↔BC5 comparison is retired"). The pairwise engine went
out with PR #308 and is recoverable from git history if a reachable legacy
backend ever returns. Keeping a gate that no operator can discharge is how a
registry entry rots into permanent-blocked; that is why this is written down
rather than quietly dropped.

**What replaced it, and what it found.** The pairwise *question* — does BC4
accept the same recurrence contract? — survives the loss of the live rail,
because the `four` branch source is still pinned at
`compatibility.bc3-four` = `9d73959a` (2026-06-12) in
[`spec/api-provenance.json`](../api-provenance.json). Diffing the three
load-bearing files between that pin and the `bc3` pin answers it offline, and
the answer is: **the write contract is identical on both rails.** The
`recurrence_schedule` permit list is byte-identical. `RecurrenceSchedule`'s
`attr_accessor` set, `attributes`, `pattern_attributes`, `window_attributes`,
and `as_json` are unchanged, so the echoed key set is the same. The recurrence
deltas are exactly #12362's tightenings (`week_instance` `(-1..4)` →
`[ -1, 1, 2, 3, 4 ]`, the new `days_must_fit_frequency`, the `days=` coercion
change), plus `ensure_valid_recurrence_schedule` narrowing from every save to
`new_record?`, plus `duration` being computed separately for the timed and
all-day branches. Nothing there changes what an SDK would model. Prefer this
source diff to a live pairwise probe permanently: it is free, offline,
reproducible from the pins, and it is the only form of the check that can still
be run.

**The BC5 half is downgraded from a hard gate to an optional post-absorption
check.** The gate existed because "the derived-fields and silent-discard
semantics make doc-only modeling risky". That premise is correct — but the
alternative to a live probe is not doc-only modeling, it is **source-at-the-pin
modeling**, which is what the rest of this registry already does. Both claims
the gate was protecting are *structural*, not behavioral, and both are settled
by the citations table above:

- **Derived fields are not "observed to be ignored", they are unreachable.**
  `hour`, `minute`, `start_date`, `duration`, and `end_date` do not appear in
  the controller's `permit` list, so strong parameters drops them before a
  `RecurrenceSchedule` is ever constructed. A probe cannot make this more true;
  it can only re-confirm it from the outside.
- **Silent discard is one `before_save` guard**, and reading it corrected two
  errors this entry had been carrying (#12362 does not produce a validation
  error, and sub-two-occurrence patterns discard as well). The probe was
  supposed to catch drift like that. Reading the source caught it instead, at
  zero cost and with no production writes.

Reading the source also caught the thing the probe was *most* likely to catch
and the doc was *least* likely to state: `spec/basecamp.smithy` does not model
`recurrence_schedule` at all, and `doc/api`'s illustrative echo is missing three
of the ten keys. A live probe would have surfaced the second of those. The
source read surfaced both.

**Verdict: doc-only modeling is not defensible here; source-at-the-pin modeling
is; the live probe is not required to start.** Absorb against the cited files,
and treat the probe as post-absorption verification if anyone wants it.

**Residual risk this accepts.** Two things the source cannot prove: the exact
JSON encoding of the derived values (`start_date`/`end_date` render as
`"YYYY-MM-DD"`; `duration` as integer seconds), and that production BC5 is
running the pinned revision. The first is cheap to model conservatively and
cheap to correct. The second is the standing assumption behind every entry in
this registry — singling this one out for a live proof is an inconsistent bar,
not a higher one.

### If someone wants the probe anyway — what it costs

Written down so this is a decision someone can approve or decline, rather than
an open-ended blocker. It is **not** a gate on absorption.

- **Credentials.** `BASECAMP_TOKEN`, `BASECAMP_ACCOUNT_ID`, `BASECAMP_HOST`
  against production BC5, plus a project the caller can write to. None are
  configured in this repo's environment.
- **No harness to ride.** All 31 cases in
  [`conformance/tests/live-my-surface.json`](../../conformance/tests/live-my-surface.json)
  are `GET` and tagged `read-only`; the live canary has no mutation lane at all.
  The probe is a bespoke script, not a canary case.
- **Coverage.** Nine creates, one per frequency family — `every_day`,
  `every_weekday`, `every_week`, `every_other_week`, `every_month`,
  `every_day_of_month`, `every_year`, `custom_week`, `custom_month` — since the
  `days`/`week_instance` derivation branches on frequency. Add three negatives:
  `week_instance: 0`, a `days` member outside the frequency's range, and a
  `recurs_until` close enough to `starts_at` to yield a single occurrence. All
  three should return `201` with `recurrence_schedule` **absent**.
- **What to diff.** The echoed `recurrence_schedule` against the ten-key table
  above — key set first (does the response really carry `week_interval`,
  `month_interval`, and `duration` as nulls?), then per-key: `days`
  canonicalization, `hour`/`minute` against the submitted `starts_at`,
  `duration` as integer seconds, `start_date`/`end_date` string format. Diff
  against the source-derived table, not against `doc/api`'s seven-key example,
  which is already known to be incomplete.
- **Blast radius and cleanup.** These are real records in a live account, and a
  recurring entry is not one artifact: it fans out into occurrences and can
  notify participants and subscribers. Two levers cut this down — create with
  `status: "drafted"` (documented, and the recurrence machinery still runs, so
  the echo is unaffected), and pass no `participant_ids`. Cleanup is a
  `DELETE` per created entry, and the operator should confirm the project's
  schedule and the account timeline are clean afterward.
- **Authorization.** Production mutations against a live account need an owner's
  sign-off before anyone runs this. Nothing in this entry constitutes that
  sign-off.
