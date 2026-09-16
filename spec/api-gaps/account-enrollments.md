---
gap: account-enrollments
status: addressed-in-bc3-pr-9962
detected: 2026-09-15
sdk_demand: medium
bc3_pr: 9962
bc3_refs:
  introduced_in: "Add API endpoint for bulk invites (BC3 #9962, merged 368a9b6edb9)"
  routes:
    - "POST /:account_id/account/enrollments/people.json (documented, doc/api/sections/people.md)"
  sections:
    - doc/api/sections/people.md
  related_existing_api:
    - UpdateProjectClientAccess
    - ListPeople
---

# Enrolling people on the account, in bulk

## What's missing

BC3 **#9962** (`368a9b6edb9`) documents
`POST /account/enrollments/people.json`, which enrolls one or more people on
the **account** and emails each new person an invitation. The SDK models no
operation for it.

The docs distinguish it from the project-access endpoint the SDK already
models: `UpdateProjectClientAccess` grants and revokes access to a single
project for people who already exist, whereas this adds people to the account
itself and can optionally grant several projects at once. They are not
substitutes.

Its parameters carry two behaviours that need modeling attention, not just
transcription:

- **`project_ids` fails open and silently.** IDs that are not integers, that
  the caller cannot manage people on, or that are not visible to the caller are
  *ignored*, and the request still succeeds. A `201` therefore does not mean
  every requested project was granted. Any SDK surface must not imply it does.
- **`notify` is a tri-state in practice.** It defaults to `true` and accepts
  `false`, `"false"`, `"0"` and `0`. Only the boolean belongs in the modeled
  request; the string forms are bc3 leniency, not contract.

An `email_address` already on the account re-enrolls that person rather than
creating a duplicate, so the operation is not cleanly idempotent but is safe to
retry in the sense that matters. The caller must be able to manage people;
otherwise `403`.

## Why it matters

Medium demand, with a sharp edge. Bulk invitation is exactly the kind of thing
scripts are written for — onboarding a contractor team, seeding a project from
a roster — and it is the one surface here where an SDK that models the happy
path carelessly would actively mislead: a caller who reads `201` as "all five
people got all three projects" has been told something bc3 never promised.

That is the argument for filing it deliberately rather than absorbing it in
passing. It needs a response projection that lets the caller see what actually
happened, and that deserves its own review.

## Suggested API shape

```
EnrollPeople: POST /{accountId}/account/enrollments/people.json -> EnrollmentResult
```

with a request of `people` (required list of `{name, email_address, title?,
company_name?}`), `project_ids` (optional list), and `notify` (optional
boolean, modeled as a boolean only).

Before modeling the output, read the jbuilder: the SDK should return what was
granted, not echo what was asked. If bc3's response does not distinguish
requested from granted projects, that is worth raising upstream rather than
papering over — see below.

## Implementation notes for BC3

One request, and it is about the silent-ignore rule. If the `201` response does
not enumerate which `project_ids` were actually granted, a caller has no way to
learn that three of their five projects were dropped except by polling each
project's people list. Returning the granted set (or the ignored set) would make
the endpoint self-describing. Until then the SDK can only document the hazard.

## SDK absorption plan when this lands

- One Smithy operation, tagged into the People service group.
- New request structures for the person entries; reuse `Person` for the
  response where the payload allows.
- Model `notify` as a boolean; do not model the string spellings.
- Conformance: a multi-person enrollment, a partial-grant case pinning that a
  `201` can coexist with ungranted projects, and the `403`.
- The operation's documentation must state the silent-ignore behaviour
  explicitly — it is the single most surprising thing about this endpoint.
- Delete this file and its `spec/bc3-route-allowlist.yml` entry when it lands.
