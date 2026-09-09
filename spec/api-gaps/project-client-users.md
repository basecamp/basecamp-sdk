---
gap: project-client-users
status: absorbed-in-sdk
detected: 2026-09-08
sdk_demand: high
bc3_pr: 13098
smithy_refs:
  - UpdateProjectClientAccess
  - EnableProjectClients
  - DisableProjectClients
  - ClientInvitationValidationError
bc3_refs:
  introduced_in: "Add clients via the API (BC3 #13098)"
  routes:
    - PUT /:account_id/projects/:project_id/people/client_users.json
    - POST /:account_id/projects/:project_id/client_enablement.json
    - DELETE /:account_id/projects/:project_id/client_enablement.json
  controllers:
    - app/controllers/projects/people/client_users_controller.rb
    - app/controllers/projects/client_enablements_controller.rb
  related_existing_api:
    - UpdateProjectAccess
    - ListProjectPeople
    - Person
---

# Project client users (grant / revoke / invite) and client enablement

## What's missing

`PUT /projects/{id}/people/users.json` (`UpdateProjectAccess`) manages team
membership only: it never grants client access, and a client's id passed to
it is dropped. Until BC3 #13098 the API had no way to admit a client to a
project or to turn on a project's client-facing surface, so an SDK client
could read `Person.client` but never produce one.

BC3 #13098 documents both halves in the People section:

- `PUT /projects/{id}/people/client_users.json` — the client-side mirror of
  `UpdateProjectAccess`: `grant` existing client ids, `revoke` client ids,
  `create` brand-new clients by email (`name` optional, defaulting to the
  address; `title` and `company_name` optional). Only client users are
  eligible: a team member's id is omitted from `granted` rather than
  cross-graded, and `revoke` never removes a team member. Response is the
  same `{granted, revoked}` people shape, each `client: true`. `403` unless
  clients are enabled on the project.
- `POST` / `DELETE /projects/{id}/client_enablement.json` — the enablement
  toggle, answering `{"clients_enabled": true|false}`. `POST` is `403` unless
  the project can have clients; `DELETE` is `403` while any client users
  remain.

## Why it matters

Onboarding a client is the first thing an agency does on a new project, and
it was web-only. A CLI or agent that can create the project, post the
messages and build the to-do lists still had to hand a human the browser for
the one step that lets the client see any of it.

## Suggested API shape

Three operations on the People service, tagged `People` beside
`UpdateProjectAccess`:

- `UpdateProjectClientAccess`: same input triad, with `CreateClientRequest`
  requiring only `email_address`. Naturally idempotent PUT. Declares
  `retryOn: [503]`, not the usual `[429, 503]`: this endpoint's 429 is the
  account seat-limit verdict (no `Retry-After`, deterministic), so a retry
  budget only delays the same answer.
- `EnableProjectClients` / `DisableProjectClients`: bodyless POST/DELETE
  returning `ProjectClientEnablement { clients_enabled }`. Both naturally
  idempotent (re-toggling re-answers the same state).

## Implementation notes for BC3

Invitations are all-or-nothing. `Projects::People::ClientUsersController`
validates every `create` row before writing (an email-less row is kept so it
fails validation instead of being silently dropped), answers
`422 {"errors": [{"email_address", "messages"}]}` naming each rejected row
(`email_address: null` for a blank one), checks capacity once for the
distinct addresses not already on the account (`429` when they would exceed
the limit; a repeated address counts once), and enrolls the batch in one
transaction. Enabling clients applies the project's default client
visibility (timeline and most docked tools shared; card table, Campfire and
Doors private), which is why admitting a client never enables the project
implicitly.

## SDK absorption plan when this lands

Absorbed here in all six SDKs from the Smithy model. The per-row `422` body is
a third validation shape beside the flat `{error}` and the field-keyed
`{errors: {field: [...]}}` map; SPEC §6 "Row-keyed validation bodies" folds
it into the shared `field_errors` slot keyed by each row's `email_address`
so the rejected addresses reach `message` and the structured slot in every
language. Conformance pins the three paths, the wire body, the 503 retries,
and the single-attempt 429. The routes are waived in
`spec/bc3-route-allowlist.yml` until the next provenance repin: the range from
the revision `spec/api-provenance.json` currently pins also carries the
Subtask, circles, bulk-enrollment and backlinks families, which are triaged
separately.
