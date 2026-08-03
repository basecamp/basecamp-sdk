---
gap: my-bookmarks
status: absorbed-in-sdk
detected: 2026-07-25
sdk_demand: medium
bc3_pr: 12383
smithy_refs:
  - "ListMyBookmarks operation"
  - "GetBookmark operation"
  - "CreateBookmark operation"
  - "DeleteBookmark operation"
  - "Bookmark structure"
  - "BookmarkStatus structure"
bc3_refs:
  introduced_in: master
  routes:
    - GET /:account_id/my/bookmarks.json
    - GET /:account_id/recordings/:recording_id/bookmark.json
    - POST /:account_id/recordings/:recording_id/bookmark.json
    - DELETE /:account_id/recordings/:recording_id/bookmark.json
  controllers:
    - app/controllers/my/bookmarks_controller.rb
    - app/controllers/recordings/bookmarks_controller.rb
  related_existing_api:
    - GetMyNotifications (personal, per-user reading surface under /my)
    - ListRecordings (the wrapped element is the shared recording projection)
---

# My bookmarks

## What's missing

SDK registration only — the contract shipped to `master` via BC3 **#12383**
("Add My Bookmarks JSON API", `640389c2`, 2026-07-25).
`doc/api/sections/my_bookmarks.md` on `master` is the contract of record. The
SDK does not yet model any of the four operations; this entry registers the
surface so the pin that absorbed `640389c2` is not carrying an unrecorded API
change.

A bookmark is a **personal** link between the current user and a single
recording (message, to-do, document, card, …); bookmarks are visible only to
their creator. Four operations:

- **`GET /my/bookmarks.json`** — a **paginated** list (Link + `X-Total-Count`,
  most-recently-bookmarked first) of bookmark envelopes. Each entry is
  `{ id, created_at, updated_at, recording }` where `recording` is the **shared
  recording projection** (the same shape `ListRecordings` returns).
- **`GET /recordings/{id}/bookmark.json`** — returns `{ "bookmarked": true }`
  or `{ "bookmarked": false }` for the current user.
- **`POST /recordings/{id}/bookmark.json`** — bookmarks the recording; returns
  **`201 Created`** with the bookmark envelope. **Idempotent**: re-bookmarking
  returns the existing bookmark, no duplicate.
- **`DELETE /recordings/{id}/bookmark.json`** — removes the bookmark; returns
  **`204 No Content`**. **Idempotent**: deleting a non-existent bookmark also
  returns `204`.

## Why it matters

Bookmarks are a first-class personal-organization surface in the BC5 web/mobile
clients. Without them a custom integration cannot list a user's saved
recordings or toggle a bookmark, and there is no client-side workaround (the
relationship is private per-user and not exposed on the recording itself beyond
the `bookmark_url` link the recording projection already carries).

## Suggested API shape

Model a new `BookmarksService` (personal `/my` + per-recording surface):

- `ListMyBookmarks` → `GET /my/bookmarks.json`, Link-paginated, element = a
  `Bookmark` envelope `{ id, created_at, updated_at, recording: Recording }`.
- `GetBookmark` → `GET /recordings/{recordingId}/bookmark.json`, returns
  `{ bookmarked: Boolean }` (a small dedicated output shape, not `Recording`).
- `CreateBookmark` → `POST /recordings/{recordingId}/bookmark.json`, `201`,
  returns the `Bookmark` envelope, **`@basecampIdempotent`** (re-POST returns
  the existing bookmark; safe to retry).
- `DeleteBookmark` → `DELETE /recordings/{recordingId}/bookmark.json`, `204`,
  no body, **`@basecampIdempotent`** (delete-of-absent is also `204`).

The wrapped `recording` reuses the existing shared recording projection — note
that projection emits `parent` only for non-docked recordings with a parent
(see [[external-links-doors]]), so a `Bookmark`'s recording must model `parent`
as optional.

## Implementation notes for BC3

Shipped — nothing pending. `my/bookmarks_controller.rb` serves the paginated
`/my/bookmarks.json` index; `recordings/bookmarks_controller.rb` serves the
singular `show`/`create`/`destroy` under `/recordings/{id}/bookmark.json`
(`resource :bookmark, only: %i[show create destroy]`). Create is `201` and
delete is `204`, both idempotent.

## SDK absorption plan when this lands

**Absorbed** (post-#504 program C5): `BookmarksService` models all four
operations with the `Bookmark` envelope and `BookmarkStatus` output; the tag
`Bookmarks` resolves to `BookmarksService` in every generator via the default
fallback (zero overrides). Both mutations are flagged idempotent and covered by
idempotency conformance cases; per-op happy-path + 4xx tests in TS/Ruby/Python,
Go wrappers with an `AccountClient.Bookmarks()` accessor, and `paths.json`
entries with dispatch in all five runners.
