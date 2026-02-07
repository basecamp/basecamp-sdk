---
name: api-sync
description: >
  Check upstream Basecamp API changes and sync the Smithy spec.
  Compares bc3-api docs and bc3 app code against tracked revisions
  in spec/api-provenance.json, identifies what changed, and optionally
  updates the Smithy spec and regenerates SDKs.
user_invocable: true
arguments:
  - name: mode
    description: "check (default), sync, or update-rev"
    required: false
---

# API Sync Skill

You are synchronizing the Basecamp SDK's Smithy spec against upstream API changes.

## Inputs

- **mode**: `{{ arguments.mode | default: "check" }}`

## Phase 1: Load State

1. Read `spec/api-provenance.json` to get the last-synced revisions for `bc3_api` and `bc3`.
2. Read `spec/README.md` to confirm upstream repo paths:
   - bc3-api: `~/Work/basecamp/bc3-api`
   - bc3: `~/Work/basecamp/bc3`

## Phase 2: Check Upstream

For **bc3-api** (API reference docs):
```bash
cd ~/Work/basecamp/bc3-api && git log --oneline <bc3_api.revision>..HEAD -- sections/
```

For **bc3** (Rails app):
```bash
cd ~/Work/basecamp/bc3 && git log --oneline <bc3.revision>..HEAD -- app/controllers/ | head -30
```

Summarize the changes by API domain (todos, messages, people, etc.). If there are no changes, report "up to date" and stop.

If mode is `check`, stop here after reporting what changed.

## Phase 3: Sync Spec (mode=sync only)

For each changed section in bc3-api:

1. Read the upstream doc: `~/Work/basecamp/bc3-api/sections/<file>.md`
2. Read the corresponding Smithy operations in `spec/basecamp.smithy` and `spec/overlays/`
3. Identify gaps: missing operations, changed fields, new parameters
4. Propose specific Smithy changes and apply after confirmation

For controller changes in bc3, cross-reference with the API docs to identify behavioral changes that affect the spec.

## Phase 4: Regenerate (mode=sync only)

After spec changes are applied:

```bash
make smithy-build
make -C go generate
make url-routes
make ts-generate && make ts-generate-services
make rb-generate && make rb-generate-services
make provenance-sync
make check
```

Fix any issues that arise during generation or checks.

## Phase 5: Update Revisions (mode=sync or update-rev)

Get the current HEAD of each upstream repo:
```bash
cd ~/Work/basecamp/bc3-api && git rev-parse HEAD
cd ~/Work/basecamp/bc3 && git rev-parse HEAD
```

Write the new revisions and today's date to `spec/api-provenance.json`:
```json
{
  "bc3_api": {
    "revision": "<new-sha>",
    "date": "<today>"
  },
  "bc3": {
    "revision": "<new-sha>",
    "date": "<today>"
  }
}
```

Then run `make provenance-sync` to update the Go embedded copy.

## Output

Report a summary of:
- What changed upstream (by domain)
- What spec changes were made (if sync mode)
- New revision SHAs stamped
- Any warnings or issues encountered
