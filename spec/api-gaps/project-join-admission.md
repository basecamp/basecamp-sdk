---
gap: project-join-admission
status: addressed-in-bc3-pr-13623
detected: 2026-10-19
sdk_demand: medium
bc3_pr: 13623
bc3_refs:
  introduced_in: "Let API clients join an all-access project (BC3 #13623, c579a5a18c), documented in people.md as Join a project"
  routes:
    - "POST /:account_id/buckets/:bucket_id/admissions.json"
  related_existing_api:
    - GetProject
    - UpdateProjectAccess
---

# Join an all-access project

## What's missing

BC3 #13623 documents `POST /buckets/{projectId}/admissions.json`, which joins
the current person to a project whose `admissions` policy lets them in (`team`,
`employee`, or `invite` for account owners only). It answers `201` with an empty
body, `302` to the project when the person already has access, and `404` when the
project is missing or its policy does not admit them. A read of a project the
person could join answers `403` with an `admission_url` and `admission_method`
instead. The SDK models neither the join nor those two fields.

## Why it matters

`GET /projects.json` lists only projects the caller already has access to, so an
agent or integration that is handed a project id it could join has no typed way
to join it. Medium demand.

## Suggested API shape

`JoinProject` (`POST /buckets/{projectId}/admissions.json`), no body, `201` with
no content, `NotFoundError` and `ForbiddenError` (an agent acting as itself is
refused), and the `302` already-a-member case documented on the operation.
`GetProject`'s `403` body gains `admission_url` and `admission_method`.

## Implementation notes for BC3

Nothing further. The `admission_url` carries no `.json` extension, so a client
that follows it must send `Accept: application/json` or add the extension.

## SDK absorption plan when this lands

A spec PR adds the operation and the `403` shape, regenerates, and adds the
per-SDK tests, including the redirect case. Its allowlist entry in
`spec/bc3-route-allowlist.yml` is deleted in the same PR.
