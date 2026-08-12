---
gap: project-archive-unarchive
status: absorbed-in-sdk
detected: 2026-08-05
sdk_demand: medium
bc3_pr: 12550
smithy_refs:
  - ArchiveProject
  - UnarchiveProject
  - ProjectLimitError
bc3_refs:
  introduced_in: "archive-project-api (BC3 #12550, merged 6f4781bbd4)"
  routes:
    - PUT /:account_id/projects/:project_id/status/archived.json
    - PUT /:account_id/projects/:project_id/status/active.json
  controllers:
    - app/controllers/projects/status_controller.rb
  related_existing_api:
    - TrashProject (the only project status transition currently modelled)
    - CreateProject (shares the 507 project-limit response body)
    - ArchiveRecording / UnarchiveRecording (the sibling status surface, already modelled)
---

# Archive and unarchive a project

## What's missing

`TrashProject` is the only project status transition the SDK models. There is
no `ArchiveProject` and no `UnarchiveProject`, so an SDK consumer can move a
project to the trash but cannot archive one, and cannot restore a project from
either the archive or the trash.

The routes are not new — `PUT /projects/{id}/status/archived.json` has worked
for as long as the controller has existed. What was missing on the BC3 side was
a `respond_to`: the controller answered a JSON request with a **302** to an HTML
URL, and following that `Location` returned **406**, because API requests can't
resolve HTML views. BC3 **#12550** adds the JSON branch (`204 No Content`) and
documents both endpoints in `doc/api/sections/projects.md`.

This is worth registering as a gap rather than a plain repin item: the contract
consumers see genuinely changes shape (302 → 204), and the docs previously let
you *read* `?status=archived` on `ListProjects` while offering no documented way
to set it.

## Why it matters

Archiving is the normal end-of-life move for a project in the product — trashing
is the destructive one, and it's the only one an SDK consumer has today. Any
integration that wraps up projects on a schedule (end of engagement, end of
quarter) has to either leave them active or trash them, which is not the same
operation and is not reversible on the same terms.

There is no client-side workaround. `UpdateProject` looks like one, but
`PUT /projects/{id}.json` with `project[status]` returns **200** and silently
drops the field — `status` is not in BC3's permit list
(`app/controllers/concerns/project_operations.rb`). That silent drop is
called out as a known adjacent defect in #12550 and deliberately left unfixed
there; the SDK should not model `status` as writable on `UpdateProject`.

BC3 **#12566** later documented exactly that, in two lines of
`doc/api/sections/projects.md`: a project's `status` is read-only on Update a
project, passing one has no effect and still returns 200, and callers are pointed
at Archive, Unarchive and Trash instead. So the SDK's decision not to model
`status` as writable is now backed by upstream prose rather than by reading the
permit list, and #12566's "see Archive/Unarchive" pointer resolves to the two
operations this brief absorbs. The underlying silent-drop behaviour is unchanged —
#12566 documents it, it does not fix it.

## Suggested API shape

Two operations on the existing projects service, modelled on `TrashProject`:

- `ArchiveProject` → `PUT /{accountId}/projects/{projectId}/status/archived.json`,
  `204`, empty output, `@idempotent` + `@basecampIdempotent(natural: true)`
  (re-archiving an archived project is a no-op that still returns 204).
- `UnarchiveProject` → `PUT /{accountId}/projects/{projectId}/status/active.json`,
  `204`, empty output, same idempotency. Note this restores from **trashed** as
  well as archived — it is the inverse of both `ArchiveProject` and
  `TrashProject`, not just the former.

Input shapes mirror `TrashProjectInput` exactly (`accountId` + `projectId`
`@httpLabel`s, nothing else — no body).

Two error-modelling notes:

- **403 is reachable on both; the asymmetry is in the cause, not in
  reachability.** `Projects::StatusController` declares `before_action
  :forbid_clients` **unscoped**, and `forbid_clients` is `head :forbidden if
  Current.user.client?` (`app/controllers/concerns/permissions.rb:22-24`), so a
  client user gets 403 from `active` exactly as from `archived`. Both operations
  therefore carry `ForbiddenError`. What is archive-only is the *second* guard:
  `ensure_account_can_archive_and_trash_projects` and
  `ensure_can_archive_or_trash_project` are both `only: %i[ archived trashed ]`,
  which is the admin/creator restriction accounts on the admin pro pack can
  enable. bc3's own tests split along exactly that line — "pro-pack forbids
  archiving to non-admin or creator" asserts 403, and "unarchive is permitted
  for a non-admin or creator" asserts the guard does not apply — so model 403 on
  both and let the guard difference live in the documentation.
- **507 on unarchive.** `UnarchiveProject` returns `507 Insufficient Storage`
  with `{"error": "The project limit for this account has been reached."}` when
  the account is at its project limit — `ensure_account_can_create_projects`,
  which `Projects::StatusController` runs `only: :active`, rendering that body
  with `status: :insufficient_storage`
  (`app/controllers/concerns/resource_limits.rb`). When this brief was written
  the SDK's only `@httpError(507)` shape was `WebhookLimitError`, too narrowly
  named to reuse. The same body was already reachable from `CreateProject`, which
  also did not model it — so a shared project-limit shape closed two gaps at once
  rather than one. See the absorption section below for what shipped.

  Do not let `ProjectLimitError` be reused for BC3 #12555's upload-version 507:
  that one is a **storage** limit with a different message ("The storage limit
  for this account has been reached."), and it needs its own shape. See
  [`upload-new-version.md`](upload-new-version.md).

`PUT /projects/{id}/status/trashed.json` also gained the JSON branch in #12550,
but it is deliberately **undocumented**: `DELETE /projects/{id}.json` remains
the documented trash path, and `TrashProject` already models it. Don't add a
second operation for the same transition.

`PUT /projects/{id}/status/deleted.json` (permanent deletion) was deliberately
left HTML-only and is not part of the API surface.

## Implementation notes for BC3

Nothing pending on the BC3 side — #12550 is merged (`6f4781bbd4`) and is inside
the revision `spec/api-provenance.json` currently pins.
`Projects::StatusController` now mirrors `Recordings::StatusController`:
`format.any(:html, :js)` redirects, `format.json` returns `head :no_content`, for
`active`, `archived`, and `trashed`. Routes and before_actions are unchanged.

Public docs: `doc/api/sections/projects.md` (bc3), which is what
`spec/bc3-routes.json` extracts from. The `basecamp/bc-api` mirror carries the
same two sections through bc-api#432, still open at the time of absorption —
irrelevant to the contract, since bc3's own `doc/api/` is the authority here.

The end-to-end proof is bc3's, and it is split across two files that prove
different things — cite both, because neither covers the other's case:

- `test/api/projects/status_controller_api_test.rb` — the 204s, the 403s, and
  the behaviour itself, including "unarchive restores a trashed project".
- `gems/saas/test/api/projects/status_controller_api_test.rb` — the **only** 507
  proof: "unarchive enforces the project limit" on a free-plan account, asserting
  `:insufficient_storage` and the exact body `ProjectLimitError` models.

## SDK absorption plan when this lands

Absorbed. `ArchiveProject` and `UnarchiveProject` are modelled in
`spec/basecamp.smithy` next to `TrashProject`, tagged `Projects` in
`spec/overlays/tags.smithy`, and generated into all six service layers; the Go
service layer gets hand-written `Archive`/`Unarchive` wrappers
(`go/pkg/basecamp/projects.go`) because `go-check-drift` reports an unwrapped
generated operation as an error.

The 507 is modelled as a new `ProjectLimitError` shape and wired into
**`UnarchiveProject` and `CreateProject`** — the same body was already reachable
from create and unmodelled, so one shape closed two gaps. Two shapes sharing 507
(`WebhookLimitError` is the other) is safe because no SDK maps status → shape;
all six switch on the status code into a fixed taxonomy. The invariant that does
matter is preserved: no operation's error list contains two shapes with the same
status. It surfaces as a generic `api_error` with `http_status: 507` (SPEC §7),
since no SDK gives 507 a named class.

`status` is deliberately **not** modelled as writable on `UpdateProject`: it is
absent from `create_project_params`
(`app/controllers/concerns/project_operations.rb`), so bc3 silently drops it.
Nothing was added to `conformance/tests/live-my-surface.json` — that is a
read-only live surface and archiving is destructive.
