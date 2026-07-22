---
gap: campfire-line-edit
status: addressed-in-bc3-pr-12359
detected: 2026-07-22
sdk_demand: medium
bc3_pr: 12359
bc3_refs:
  introduced_in: five
  routes:
    - "PUT /:account_id/chats/:chat_id/lines/:line_id.json"
    - "PUT /:account_id/buckets/:project_id/chats/:chat_id/lines/:line_id.json (legacy project-scoped)"
  controllers:
    - app/controllers/chats/lines_controller.rb
  related_existing_api:
    - GetCampfireLine
    - CreateCampfireLine
    - DeleteCampfireLine
---

# Campfire line edit

## What's missing

SDK absorption only — the contract shipped via BC3 **#12359** (merged
2026-07-22, post-train follow-up to the BC5 API docs). "Update a Campfire
line" in `doc/api/sections/campfires.md` on `master` is the contract of
record:

- `PUT /chats/:chat_id/lines/:line_id.json` (canonical account-scoped form;
  the project-scoped `PUT /buckets/:project_id/chats/:chat_id/lines/:line_id.json`
  is documented under Legacy project-scoped routes).
- **Required parameter**: `content` — the new body for the line.
- Only text and rich text lines can be edited, and **only by their creator**.
- The new content is treated as rich text: the line becomes a rich text line
  even if it was originally plain text, and `content` may include the HTML
  tags covered in the Rich text guide.
- Returns `204 No Content` on success, `403 Forbidden` if the current user
  isn't allowed to edit the line.

The same PR also tightened the documented **delete** contract (already
modeled as `DeleteCampfireLine`): lines can be deleted by their creator *or
by an admin*, and `403 Forbidden` is now documented for the disallowed case.
That's a doc-comment refinement on the existing operation, not a new gap.

## Why it matters

Line editing is a BC5 capability with no BC4 analog. Without it, an SDK
consumer that posts Campfire lines (bots, bridges, notifiers) can't correct
a message after the fact — the only recourse is delete + repost, which loses
position and reads as churn in the room.

## Suggested API shape

`UpdateCampfireLine` operation: `PUT /{accountId}/chats/{chatId}/lines/{lineId}.json`
with a required `content` body member, `204` response (no output payload),
and `ForbiddenError` in the error list. Input mirrors `CreateCampfireLine`'s
content member; no response-shape work needed.

## Implementation notes for BC3

Shipped — nothing pending. `chats/lines_controller.rb` serves the route;
the doc documents request/response and the permission model (creator-only
edit; creator-or-admin delete).

## SDK absorption plan when this lands

- **SDK PR #295 (open) already models `UpdateCampfireLine`** and is the
  likely absorbing PR; the §Q absorption queue's PR-2 build-ahead pair is
  the fallback vehicle if #295 stalls.
- Status flips to `absorbed-in-sdk` with the absorbing PR (which adds the
  Smithy refs).
- While absorbing, refresh `DeleteCampfireLine`'s doc comment with the
  creator-or-admin / 403 language from the same doc section.
- Pairwise check: route 404s on BC4 (no line editing), succeeds on BC5 —
  additive-only, no invariant violation.
