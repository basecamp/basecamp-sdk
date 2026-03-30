---
name: api-sync
description: >
  Check upstream Basecamp API changes and sync the Smithy spec.
  Compares bc3 API docs and app code against the tracked bc3 revision
  in spec/api-provenance.json, identifies what changed, and optionally
  updates the Smithy spec and regenerates SDKs.
disable-model-invocation: true
argument-hint: "[check|sync|update-rev]"
---

# API Sync Skill

You are synchronizing the Basecamp SDK's Smithy spec against upstream API changes.

## Inputs

- **mode**: `{{ arguments.mode | default: "check" }}`

## Upstream repo

- **bc3** (canonical source): `basecamp/bc3`
  - API reference docs: watch `doc/api/`
  - Rails app/API implementation: watch `app/controllers/`

The public `basecamp/bc3-api` repo is a mirror for documentation consumption, not a provenance input.

## Phase 1: Load State

1. Read `spec/api-provenance.json` to get the last-synced revision for `bc3`.

## Phase 2: Check Upstream

List files changed in the watched paths since the last sync.

For **bc3 API docs** (`doc/api/`):
```bash
gh api repos/basecamp/bc3/compare/<bc3.revision>...HEAD \
  --jq '[.files[] | select(.filename | startswith("doc/api/"))] |
    if length == 0 then "  (no changes in doc/api/)"
    else .[] | .status[:1] + " " + .filename
    end'
```

For **bc3 API implementation** (`app/controllers/`):
```bash
gh api repos/basecamp/bc3/compare/<bc3.revision>...HEAD \
  --jq '[.files[] | select(.filename | startswith("app/controllers/"))] |
    if length == 0 then "  (no changes in app/controllers/)"
    else .[] | .status[:1] + " " + .filename
    end'
```

Summarize the changed files by API domain (todos, messages, people, etc.). If there are no changes in either watched path, report "up to date" and stop.

If mode is `check`, stop here after reporting what changed.

## Phase 3: Sync Spec (mode=sync only)

For each changed doc file in `bc3`:

1. Fetch the upstream doc from `basecamp/bc3`:
   ```bash
   gh api repos/basecamp/bc3/contents/doc/api/<path> --jq '.content' | base64 -d
   ```
2. Read the corresponding Smithy operations in `spec/basecamp.smithy` and `spec/overlays/`
3. Identify gaps: missing operations, changed fields, new parameters
4. Propose specific Smithy changes and apply after confirmation

For controller changes in `bc3`, cross-reference them with `doc/api/` to identify behavioral changes that affect the spec.

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

## Phase 5: Update Revision (mode=sync or update-rev)

Get the current `bc3` HEAD:
```bash
gh api repos/basecamp/bc3/commits/HEAD --jq '.sha'
```

Write the new revision and today's date to `spec/api-provenance.json`:
```json
{
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
- New revision SHA stamped
- Any warnings or issues encountered
