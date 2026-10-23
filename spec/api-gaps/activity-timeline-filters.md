---
gap: activity-timeline-filters
status: absorbed-in-sdk
detected: 2026-10-23
sdk_demand: high
smithy_refs:
  - "GetProgressReportInput.date (spec/basecamp.smithy)"
  - "GetProgressReportInput.projectIds (spec/basecamp.smithy)"
  - "GetProgressReportInput.peopleIds (spec/basecamp.smithy)"
bc3_refs:
  introduced_in: "BC4-era timeline; params read by Timeline::Filters"
  routes:
    - GET /:account_id/reports/progress.json
  controllers:
    - app/controllers/timelines_controller.rb
    - app/controllers/concerns/timeline/filters.rb
    - app/controllers/concerns/timeline/date_parsing.rb
    - app/controllers/concerns/timeline/activity_view_scoped.rb
  related_existing_api:
    - GetProgressReport
---

# Activity timeline: the day, project and people parameters

## What's missing

`GET /reports/progress.json` reads three query parameters that
`doc/api/sections/timeline.md` does not mention: `date`, `project_ids[]` and
`people_ids[]`. They are what the web's activity view sends for its date
stepper and its "Showing [All projects] by [Everyone]" dropdowns. Read in bc3
at `9c09e30bc9` (master, 2026-10-23); not yet documented there.

- **`date`.** `Timeline::Filters#set_filter` passes it to `Timeline::Filter`
  as `from_date`, and `to_date` defaults to the same day, so the endpoint
  serves **one day per request**. Without it the day is `Date.current`. Paging
  stays inside the day: unfiltered paging runs out at midnight and later pages
  are empty. `Timeline::DateParsing#parse_date` reads `today` or any value
  `Date.parse` rejects as today, silently.
- **`project_ids[]`, `people_ids[]`.** `set_filter_settings` copies each onto
  the filter settings, `Array(...).map(&:to_i)`, so only the bracketed
  repeated form arrives as an array. Both combine, and `page` composes with
  them.

Two server behaviours every caller has to know:

1. **Omitted is not "everything".** `Timeline::ActivityViewScoped` builds the
   settings from the caller's saved `User::ActivityTimelineSetting`, and each
   parameter overrides only its own dimension. A call with no `project_ids[]`
   gets the projects the caller last picked in the web's dropdown.
2. **No visible ID clears the saved filter.** `set_filter_projects` and
   `set_filter_people` call `reset_later` when the filter names IDs and none
   of them is visible to the caller. That enqueues
   `User::Settings::ResetActivityTimelineJob`, which empties both saved
   lists. The request that triggered it returns no events. A GET with a stale
   ID therefore changes the user's web view.

`GetProjectTimeline` and `GetPersonProgress` do not read these parameters, so
they stay on `GetProgressReport` only.

## Why it matters

Without `date` an SDK caller can read only today's activity. The CLI's TUI
activity feed hits exactly this: it pages to midnight and stops. Without the
two ID lists a caller can't narrow the feed the way the web does, and gets
whatever the user last saved in the web instead.

## Suggested API shape

What the controller already does, documented on `GET /reports/progress.json`:

- `date` (`YYYY-MM-DD`, optional, default today): the day to return.
- `project_ids[]` (repeated, optional): only events in these projects.
- `people_ids[]` (repeated, optional): only events created by these people.
- That omitted ID lists fall back to the saved web filter, and that a list
  with no visible ID clears it.

## Implementation notes for BC3

Add the three parameters, and both behaviours above, to the "Get timeline"
section of `doc/api/sections/timeline.md`. The second behaviour is arguably a
bug for API callers, since a read resets stored state; worth deciding whether
the reset should be limited to HTML requests.

## SDK absorption plan when this lands

Absorbed ahead of the bc3 doc, since the route is already documented and
modelled and only its query parameters are new.

- `GetProgressReportInput` gains `date` (`ISO8601Date`), `projectIds`
  (`ProjectIdList`, wire name `project_ids[]`) and `peopleIds`
  (`PersonIdList`, wire name `people_ids[]`), regenerated into all seven
  SDKs. Each member's doc carries both server behaviours.
- Go: `TimelineService.Progress` takes a new `*ProgressOptions` (Limit, Page,
  Date, ProjectIDs, PeopleIDs) in place of `*TimelineListOptions`, which
  `ProjectTimeline` and `PersonProgress` keep. It refuses an impossible
  `Date` and a non-positive ID with a usage error before the wire, the two
  inputs bc3 would misread as today or as a filter-clearing ID. See
  `MIGRATING.md`.
- Tests assert the decoded wire query in Go, TypeScript, Ruby, Python, Rust,
  Kotlin and Swift, and that an unfiltered call sends no query.
