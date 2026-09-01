---
gap: recent-projects
status: absorbed-in-sdk
detected: 2026-08-31
sdk_demand: medium
bc3_pr: 13043
smithy_refs:
  - ListRecentProjects
  - RecordProjectVisit
bc3_refs:
  introduced_in: "Expose recently visited projects in the API (BC3 #13043)"
  routes:
    - GET /:account_id/my/recent_projects.json
    - POST /:account_id/projects/:project_id/recent_visit.json
  controllers:
    - app/controllers/my/recent_projects_controller.rb
    - app/controllers/projects/recent_visits_controller.rb
  related_existing_api:
    - Project
    - ListProjects
    - GetProject
---

# Recently visited projects

## What's missing

BC5's home page is built on a per-user recent-visit log, but the API had no way
to read it or feed it. BC3 #13043 documents both halves:
`GET /my/recent_projects.json` lists the projects the current user has most
recently visited, most recent visit first, and
`POST /projects/{id}/recent_visit.json` — which bc3 has answered `204` for API
clients all along — records a visit, so CLI/SDK-driven project opens feed the
same list the web does.

The list reads the raw visit log, not the home grid's pinned-exclusion and
padding: capped at the 50 most recent visits, keeping only active projects the
person can still access. Each entry is the standard project projection plus
`bookmarked` — the wire omits `starred` here, unlike `GET /projects.json`,
which is fine because `Project.starred` is optional.

## Why it matters

A recency-ordered project list is the natural default surface for any
interactive client — "which project did I mean?" is almost always answered by
the handful visited last. Without the read, an SDK client can only fetch all
projects and guess; without the write, a CLI session's activity never
influences the list the person sees on the web.

## Suggested API shape

Two operations on the existing Projects service:

- `ListRecentProjects`: `GET /my/recent_projects.json`, no parameters, `200`
  with the project-projection array plus `bookmarked`. Not paginated (the log
  is capped at 50), so no pagination traits.
- `RecordProjectVisit`: `POST /projects/{projectId}/recent_visit.json`, no
  request body, `204`. Re-recording a visit refreshes the same entry, so the
  POST is naturally idempotent and safe to retry. Visits to archived or
  trashed projects are accepted but not recorded; an inaccessible project
  answers `404`.

## Implementation notes for BC3

BC3 #13043 adds `My::RecentProjectsController#index` over the existing
per-user visit store (`recently_visited_buckets`, merged against the person's
active, accessible project buckets) with an etag over the visit log, and
documents the long-standing `Projects::RecentVisitsController#create`. Bucket
customizations are preloaded so the fragment-cached project partial doesn't
fan out one query per entry.

## SDK absorption plan when this lands

Absorbed here as `projects.list_recent_projects` and
`projects.record_project_visit` (per-language casing) in all six SDKs. Smithy
owns both wire operations; generated service layers call the documented
routes, and Go's hand-written `ListRecent`/`RecordVisit` wrappers call only
the generated methods. Cross-SDK tests pin the paths, the bookmarked-only
projection, the visit-order decoding, the `404`, and the retry behavior of
the naturally idempotent POST. The shared `spec/fixtures/projects/recent.json`
fixture carries the documented shape.
