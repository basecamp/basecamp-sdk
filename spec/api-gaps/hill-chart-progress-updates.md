---
gap: hill-chart-progress-updates
status: absorbed-in-sdk
detected: 2026-10-19
sdk_demand: medium
bc3_pr: 13620
smithy_refs:
  - ListHillChartVersions
  - GetHillChartVersion
  - HillChartVersion
  - HillChartVersionDot
  - HillChartVersionId
  - HillChart.versions_url
bc3_refs:
  introduced_in: "Serve hill chart progress updates as JSON (BC3 #13620, fd7f569808)"
  routes:
    - "GET /:account_id/todosets/:todoset_id/hills/versions.json"
    - "GET /:account_id/todosets/:todoset_id/hills/versions/:id.json"
  related_existing_api:
    - GetHillChart
    - UpdateHillChartSettings
    - ListComments
---

# Hill chart progress updates

## What's missing

BC3 #13620 documents the progress updates behind a hill chart in
`doc/api/sections/hill_charts.md`: a paginated list, newest first, and a read by
id. Each update carries the full `description` as rich text, the `dots` (where
each tracked to-do list sat when the update was made), and `comments_count` and
`comments_url`. The hill chart read gains `versions_url` when the chart has any
updates. The SDK models the hill chart and its settings write, and no operation
for the updates themselves.

## Why it matters

The progress report already names these updates: an event of kind
`hill_version_created` has a `url` that returns the full update, and the SDK has
no typed read for that URL. Medium demand: weekly-review and agent
summarization flows reach the note text only through it.

## Suggested API shape

`ListHillChartVersions` (`GET /todosets/{todosetId}/hills/versions.json`,
paginated) and `GetHillChartVersion` (`GET
/todosets/{todosetId}/hills/versions/{versionId}.json`), tagged beside the hill
chart operations, with a `HillChartVersion` shape from the documented example.

## Implementation notes for BC3

Nothing further: docs and example markers landed in #13620.

## SDK absorption plan when this lands

A spec PR adds the two operations, the shape and `versions_url` on `HillChart`,
regenerates, and adds the per-SDK happy-path and 404 tests. Its allowlist entry
in `spec/bc3-route-allowlist.yml` is deleted in the same PR.

## Absorbed

`ListHillChartVersions` and `GetHillChartVersion` on the `HillCharts` service
(`listVersions` and `getVersion`; Go `HillCharts().ListVersions` and
`GetVersion`), the `HillChartVersion` and `HillChartVersionDot` shapes, and
`versions_url` on `HillChart`. A progress update's dot has its own shape
because its `id` is null, and `url` and `app_url` absent, once the todolist is
permanently deleted or moved to another project; `HillChartDot` keeps its
required `id`. The documented examples are shared fixtures under
`spec/fixtures/hill_charts/`, with one more carrying a dot whose todolist is
gone, and `conformance/tests/hill_chart_versions.json` runs both operations in
every SDK. The two route-allowlist entries are deleted.
