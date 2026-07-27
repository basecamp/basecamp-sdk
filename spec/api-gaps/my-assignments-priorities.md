---
gap: my-assignments-priorities
status: addressed-in-bc3-pr-12380
detected: 2026-07-26
sdk_demand: medium
bc3_pr: 12380
bc3_refs:
  introduced_in: master
  routes:
    - POST /:account_id/my/priorities.json
    - DELETE /:account_id/my/priorities/:id.json
    - POST /:account_id/my/priority_moves.json
  controllers:
    - app/controllers/my/priorities_controller.rb
    - app/controllers/my/priorities/moves_controller.rb
  related_existing_api:
    - GetMyAssignments (the retrieval side; already modeled — priorities/non_priorities)
    - CompleteTodo / UpdateCard (acting on an assignment routes to the resource endpoints)
---

# My assignments — Up Next priority management

## What's missing

SDK registration only — the contract shipped to `master` via BC3 **#12380**
("My Tasks: harden Up Next reorder and document the assignment API",
`c3086931`, 2026-07-26). `doc/api/sections/my_assignments.md` on `master` is the
contract of record. The **retrieval** side (`GET /my/assignments.json` and its
completed/due variants) is already modeled as `GetMyAssignments`; #12380 hardens
and **documents** the **Up Next priority-management** writes, which the SDK does
not model. This entry registers that net-new write surface so the provenance pin
(`c3086931`) is not carrying an unrecorded API change.

"Up Next" is the current user's ordered list of prioritized assignments
(returned as `priorities` by `GetMyAssignments`). Three write operations, all
identifying the item by its **recording id** (the id that carries the priority —
for a normalized card-table step already prioritized, the entry's
`priority_recording_id`):

- **`POST /my/priorities.json`** — *prioritize*: add a recording to Up Next.
  Body `{ "id": <recording_id> }`. Returns **`204 No Content`**.
- **`DELETE /my/priorities/{id}.json`** — *deprioritize*: remove a recording
  from Up Next. Returns **`204 No Content`**.
- **`POST /my/priority_moves.json`** — *reorder*: move an already-prioritized
  recording to a new 1-based spot. Body `{ "source_id": <id>, "position": <n> }`.
  Returns **`204 No Content`**.

The reorder endpoint has a hardened, documented **error contract** (the point of
#12380 — it previously passed `position` straight into `Array#insert`):

| Condition | Status | Body |
|---|---|---|
| Missing `position` | `400` | `{ "error": "Position is required." }` |
| Non-integer `position` | `400` | `{ "error": "Position must be an integer." }` |
| `position` outside the list | `422` | `{ "error": "Position must be between 1 and N." }` |
| Recording not prioritized | `422` | `{ "error": "Recording is not prioritized." }` |
| Inaccessible recording | `404` | (bare, no body) |

## Why it matters

Up Next is a primary BC5 personal-organization surface. A custom integration
can already *read* the ordered `priorities` via `GetMyAssignments` but cannot
add, remove, or reorder entries — there is no resource-level workaround (the
priority relationship lives on the person, not the recording).

## Suggested API shape

Add the three priority-management operations, likely onto the existing
`MyAssignmentsService`:

- `PrioritizeAssignment` → `POST /my/priorities.json`, body `{ id }`, `204`.
- `DeprioritizeAssignment` → `DELETE /my/priorities/{id}.json`, `204`.
- `ReorderUpNext` → `POST /my/priority_moves.json`, body `{ source_id, position }`,
  `204`, with the documented 400/422/404 error contract modeled as typed errors.

## Implementation notes for BC3

Shipped — nothing pending. `my/priorities_controller.rb` serves prioritize
(`create`, `POST /my/priorities.json`) and deprioritize (`destroy`,
`DELETE /my/priorities/{id}.json`); `my/priorities/moves_controller.rb` serves
reorder (`create`, `POST /my/priority_moves.json`). #12380 hardened the reorder
path (strict integer parsing → 400; range/unprioritized checks under
`person.with_lock` → 422; inaccessible → bare 404).

On idempotency for absorption: `POST /my/priorities.json` is naturally
idempotent (re-prioritizing is a no-op reorder-to-same); `DELETE
/my/priorities/{id}.json` is idempotent (delete-of-absent still `204`). `POST
/my/priority_moves.json` is a positional move — **not** retry-safe (the target
position's meaning shifts as the list changes), so it should NOT carry
`@basecampIdempotent`. Confirm prioritize/deprioritize idempotency against the
controller before gating per SPEC §7.

## SDK absorption plan when this lands

Track as a dedicated absorption PR on top of this provenance sync: add the three
Up Next write operations onto `MyAssignmentsService`, model the `204` responses
and the reorder error body as typed errors, Go wrappers, and per-op happy-path +
4xx tests in TS/Ruby/Python plus conformance `paths.json` entries. This entry
flips to `absorbed-in-sdk` with `smithy_refs` when that PR lands.
