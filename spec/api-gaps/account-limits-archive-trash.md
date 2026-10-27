---
gap: account-limits-archive-trash
status: addressed-in-bc3-pr-14076
detected: 2026-10-27
sdk_demand: medium
bc3_pr: 14076
bc3_refs:
  introduced_in: "Say in the account API whether the plan can archive and trash projects (BC3 #14076, 587912956e4, merged e08cee7fa87)"
  routes:
    - GET /:account_id/account.json
  views:
    - app/views/api/accounts/_account.json.jbuilder
  related_existing_api:
    - GetAccount
    - ArchiveProject
    - TrashProject
---

# `limits.can_archive_and_trash_projects` on the account

## What's missing

BC3 #14076 adds `can_archive_and_trash_projects` to the account's `limits`
object, beside `can_create_projects`, `can_pin_projects`, `can_create_users`
and `can_upload_files`. `account.md` documents it as "whether the plan can
archive projects and put them in the trash". When it is `false`, archiving a
project returns `403 Forbidden`, and trashing a project "deletes it
permanently instead: it can't be restored from the trash". `projects.md` says
the same under "Archive a project" and "Trash a project".

The SDK's `AccountLimits` structure models the other four booleans and not
this one, so no SDK can read it. `ArchiveProject` and `TrashProject` do not
mention the plan limit either.

## Why it matters

`TrashProject` is the one call whose effect the field changes from
recoverable to permanent. Without the field, an SDK consumer, or an agent
driving the MCP server's trash tool, cannot tell before the call which of the
two will happen. Medium demand: the field is read-only and small, but the
failure it prevents loses data.

## Suggested API shape

`can_archive_and_trash_projects: Boolean` on `AccountLimits`, documented as
the plan limit above. Extend `ArchiveProject`'s doc with the plan `403`, and
`TrashProject`'s with the permanent delete when the field is `false`.

## Implementation notes for BC3

Nothing further.

## SDK absorption plan when this lands

A spec PR adds the member and the two operation notes, regenerates, adds the
field to the `GetAccount` fixtures and tests, and records the Go and Kotlin
struct addition in MIGRATING.md as #1057 did for `Project.logo`. No route
changes, so the route allowlist is untouched.
