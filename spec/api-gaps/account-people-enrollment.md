---
gap: account-people-enrollment
status: addressed-in-bc3-pr-9962
detected: 2026-09-15
sdk_demand: medium
bc3_pr: 9962
bc3_refs:
  introduced_in: "Add API endpoint for bulk invites (BC3 #9962, 368a9b6edb9); Queenbee sync coalesced by BC3 #13134"
  routes:
    - "POST /:account_id/account/enrollments/people.json"
  controllers:
    - app/controllers/accounts/enrollments/people_controller.rb
  related_existing_api:
    - UpdateProjectAccess
    - UpdateProjectClientAccess
    - Person
---

# Account enrollment — invite people to the account in bulk

## What's missing

Until BC3 #9962 the API could grant and revoke *project* access
(`UpdateProjectAccess`, `UpdateProjectClientAccess`) but could not add anyone
to the *account*. `POST /account/enrollments/people.json` enrolls one or more
people at once: a required `people` array (each with `name` and
`email_address`, optional `title` and `company_name`), optional `project_ids`
to grant afterwards, and optional `notify` (default `true`) controlling the
invitation email. It answers `201 Created` with an array in request order,
each entry the full person representation plus a `status` of `"created"` or
`"existing"` — an address already on the account re-enrolls that person
rather than duplicating them. Enrollment is all-or-nothing: a bad entry
fails the batch with `422` and an `errors` array keyed by the entry's index
(`{ "errors": [{ "index": 1, "messages": [...] }] }`); a missing or
non-array `people` answers `422` with `{ "error": "..." }`; a caller who
cannot manage people gets `403`; a non-JSON request gets `406`. `project_ids`
the caller cannot manage, or cannot see, are silently skipped — a `201` does
not guarantee every requested project was granted. Documented in
`doc/api/sections/people.md`.

The SDK models no operation for it.

## Why it matters

Onboarding automation — HR systems, agents provisioning a team — has had to
fall back to the web enrollment flow. Medium demand: the project-access
endpoints cover the common case once people exist; this is the missing step
before them.

## Suggested API shape

`EnrollPeople`: `POST /{accountId}/account/enrollments/people.json`,
`code: 201`, input `people: EnrollmentList` (`name`, `email_address`
required; `title`, `company_name` optional), `project_ids: ProjectIdList`,
`notify: Boolean`; output a list of `EnrolledPerson` — `Person` plus a
required `status` string (`created|existing`). Not idempotent by nature
(re-sending re-enrolls rather than duplicates, but sends no second
invitation), so the 2-attempt create retry policy. Errors `ValidationError`
(both 422 shapes), `ForbiddenError`, `UnauthorizedError`, `RateLimitError`,
`InternalServerError`, and a `507 Insufficient Storage` shape for the
account's user limit — `{ "error": "The user limit for this account has been
reached." }` when the batch would create at least one new person; re-enrolling
only existing people succeeds at the limit — modelled like `StorageLimitError`
and `ProjectLimitError` so it maps to `limit_exceeded`, not a retryable
server error. The 406 is a client bug, not a contract branch. Tag it `People`.

## Implementation notes for BC3

Nothing further for the contract. Two things a modeller should know: the
indexed `errors` shape is the documented per-entry failure and differs from
the `{ "error": "..." }` scalar the parameter-missing path returns, so
`ValidationError` decoding has to accept both; the 507 is raised both by the
preflight capacity check and at enrollment time, and bc3's API tests assert
it in both places; and #13134 only coalesced the Queenbee sync — no wire
change.

## SDK absorption plan when this lands

A spec PR adds the operation, the two input structures and the
`EnrolledPerson` output, regenerates, and adds the per-SDK happy-path, 422
and 403 tests with a `spec/fixtures/people/enrollments.json` fixture built
from the documented example and an operation entry in the manifest. Go gains
`PeopleService.Enroll`. The CLI can then grow `people add --account` or a
`people invite` verb on top of it.
