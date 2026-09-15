---
gap: recording-backlinks
status: addressed-in-bc3-pr-13121
detected: 2026-09-15
sdk_demand: medium
bc3_pr: 13121
bc3_refs:
  introduced_in: "Serve a recording's backlinks as JSON to API clients (BC3 #13121, merged b6ab4d76b1b)"
  routes:
    - "GET /:account_id/recordings/:id/backlinks.json (documented, doc/api/sections/backlinks.md)"
    - "GET /:account_id/buckets/:bucket_id/recordings/:id/backlinks.json (documented alias)"
  sections:
    - doc/api/sections/backlinks.md
  related_existing_api:
    - ListRecordings
---

# Backlinks — which recordings reference this one

## What's missing

BC3 **#13121** (`b6ab4d76b1b`) adds `doc/api/sections/backlinks.md` and serves
`GET /recordings/2/backlinks.json`: the most recent 20 recordings that reference
a given recording in their rich text, newest first. This is the "references to
this" list Basecamp shows in the UI. The SDK models no operation for it.

Two properties of the endpoint matter more than its shape:

- **Each entry is the referencing recording, not a backlink object.** The docs
  say so explicitly. There is no `Backlink` resource to model — the payload is
  a list of the generic recording envelope the SDK already has.
- **The result is permission-scoped per caller.** Only backlinks from projects
  and pings the current user can access are returned, so two people reading the
  same recording's backlinks legitimately see different lists. That is a
  property to document on the operation, not a bug to reconcile.

The docs bullet a fixed page of 20 and do not describe pagination parameters,
so whether this surface paginates at all needs checking against the controller
before an operation claims `@paginated`.

## Why it matters

Medium demand. Backlinks are how a client answers "what would break if I
archived this?" or "where was this decision referenced?" — questions that
otherwise require a full-text search across the account. It composes directly
with [[recording-generic-show]]: backlinks hands you recordings of mixed type,
and the generic show route is how you resolve one you do not recognise.

Nothing is broken by its absence; it is new surface, not drift. It is filed
rather than absorbed because this PR's scope is the template library, and a
new read surface deserves its own fixtures and its own review.

## Suggested API shape

```
ListRecordingBacklinks: GET /{accountId}/recordings/{recordingId}/backlinks.json -> RecordingList
```

Reusing the existing `Recording` structure. Confirm before modeling:

- whether the 20-item cap is a page (Link headers present) or a hard limit —
  if it paginates, the operation takes the standard pagination traits, and if
  it does not, the cap belongs in the operation's documentation so callers do
  not write a loop that never terminates.
- whether the bucket-scoped alias is the canonical draw, as it is for
  several other recording sub-resources.

## Implementation notes for BC3

Nothing required. If the cap is intended as a permanent non-paginated limit
rather than a first page, `doc/api/sections/backlinks.md` should say so — the
sentence "the most recent 20" reads either way, and the SDK has to pick one.

## SDK absorption plan when this lands

- One Smithy operation plus a tag; Recordings already has a service group.
- No new structures expected — the entries are `Recording`.
- Fixtures: a populated list and an empty one, plus a case pinning that entries
  carry `type` and `parent`, since resolving them is the point of the endpoint.
- Document the per-caller visibility on the operation, so it is not read as
  nondeterminism.
- Delete this file and its two `spec/bc3-route-allowlist.yml` entries in the
  absorbing PR.
