---
gap: my-drafts
status: absorbed-in-sdk
detected: 2026-07-26
sdk_demand: medium
bc3_pr: 12381
smithy_refs:
  - "ListMyDrafts operation"
  - "Draft structure"
  - "DraftBucket structure"
  - "DraftParent structure"
bc3_refs:
  introduced_in: master
  routes:
    - GET /:account_id/my/drafts.json
  controllers:
    - app/controllers/my/drafts_controller.rb
  views:
    - app/views/api/my/drafts/index.json.jbuilder
  related_existing_api:
    - GetMyAssignments (the sibling personal "/my" aggregate; same 250 cap)
    - CreateMessage / CreateDocument (the draft/publish lifecycle this list feeds)
---

# My drafts

## What's missing

SDK registration only — the contract shipped to `master` via BC3 **#12381**
("Add My Drafts API and document the draft publication lifecycle", `123b2320`,
2026-07-26). `doc/api/sections/drafts.md` on `master` is the contract of record.
The SDK does not yet model the operation; this entry registers the surface so
the provenance pin (`c3086931`) is not carrying an unrecorded API change.

A draft is a message, document, upload, or client approval/correspondence that
has been saved but not yet published. One operation:

- **`GET /my/drafts.json`** — a **paginated** list (Link + `X-Total-Count`, most
  recently updated first) of the current user's drafts across their active
  projects, regardless of how they were authored. Drafts under an archived or
  trashed project are excluded; the API list is capped at 250 (matching
  `/my/assignments`).

Each element is a flat draft envelope (NOT the shared recording projection):

```
{ id, app_url, title, type, bucket{id,name,app_url},
  parent{id,title,app_url}|null, excerpt, created_at, updated_at,
  scheduled_posting_at|null }
```

- `type` is the short recordable name (`message`, `document`, `upload`,
  `client_approval`, `client_correspondence`).
- `parent` is `null` for drafts filed directly under their bucket.
- `excerpt` is up to 300 chars of plain text, or `""` when the draft has no body.
- `scheduled_posting_at` is `null` unless the draft is scheduled to publish later.

Five draft kinds are returned (messages, documents, uploads, client approvals,
client correspondences). **Not** returned: Google documents, cloud files, and
schedule entries — this is not an exhaustive unpublished feed.

## Why it matters

Drafts are a first-class personal surface in the BC5 clients. Without this a
custom integration cannot enumerate a user's unpublished work; there is no
client-side workaround (drafts are private per-user and not exposed on the
resource endpoints).

## Suggested API shape

Model a new `DraftsService` with a single operation:

- `ListMyDrafts` → `GET /my/drafts.json`, Link-paginated (`X-Total-Count`, most
  recently updated first), element = a dedicated `Draft` envelope
  `{ id, app_url, title, type, bucket, parent, excerpt, created_at, updated_at,
  scheduled_posting_at }` with `parent` and `scheduled_posting_at` nullable. The
  `Draft` envelope is NOT the shared recording projection — it is a flat,
  purpose-built shape.

## Implementation notes for BC3

Shipped — nothing pending. `my/drafts_controller.rb` serves the paginated
`/my/drafts.json` index via `app/views/api/my/drafts/index.json.jbuilder`.

#12381 also documented the publish path on the existing create/update endpoints
(no new operation — a behavior note on `CreateMessage`/`CreateDocument`):
a JSON create omitting `status` is silently **drafted**; publishing is an update
with `status: "active"`. A **message** update merges (publish without resending
content); a **document** update replaces (publish must resend title + content; a
status-only update is a `400`). These are documentation clarifications to
already-modeled operations, not new surface.

## SDK absorption plan when this lands

**Absorbed** (post-#504 program C6): `DraftsService` models `ListMyDrafts`
with the flat `Draft` envelope. `parent` and `scheduled_posting_at` are modeled
**required-and-nullable** (present on every element, null when bucket-rooted /
unscheduled): the scalar uses the established `["string","null"]` jsonAdd, and
the nullable `parent` object uses `anyOf: [$ref, {type: "null"}]` — a new
treatment this PR taught the Kotlin, Swift, and Python generators (TS and Go
already handled it), so all six SDKs type `parent` as nullable-required. Go
wrapper + accessor, per-SDK tests incl. present-null fixtures, and a
`paths.json` case whose mock is bucket-rooted to pin the null decode.
