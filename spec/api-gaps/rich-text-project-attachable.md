---
gap: rich-text-project-attachable
status: no-json-contract
detected: 2026-05-01
sdk_demand: low
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3e
  routes:
    - "(no new routes — change is to the rich-text attachable schema documented at doc/api/sections/rich_text.md)"
  controllers: []
  related_existing_api:
    - "Rich text rendering: Document, Message, Comment payloads"
---

# Project as Attachable in rich text

## What's missing

The existing `doc/api/sections/rich_text.md` enumerates Attachable types
(types of recordings that can be inlined in a rich-text body). BC5 adds
`Project` to that list — you can now attach a project reference inside a
message, document, or comment body.

## Why it matters

If SDK consumers rendering rich text don't model `Project` as an Attachable,
they'll see unrecognised `<bc-attachment sgid="..." content-type="...">` tags
and either drop them or render them as raw HTML. Adding it to the typed
attachable enum lets clients dispatch correctly.

## Suggested API shape

Additive: `"Project"` is a new value in the `attachableTypes` enum used by
the rich-text contract. Existing rich-text payloads gain attachments with
`content-type="application/vnd.basecamp.project"` (or whatever sgid type
the server emits).

## Implementation notes for BC3

- Update `doc/api/sections/rich_text.md` to include `Project` in the
  Attachable list.
- Confirm the `content-type` MIME string emitted; the SDK enum needs to
  match exactly.

## SDK absorption plan when this lands

- Update the rich-text Smithy / TypeScript / Ruby reference enum if one
  exists; otherwise this is a doc-only update referencing the BC3 doc.
- Comment update on any rich-text helper that enumerates attachable types.
- No service or operation changes.
- No canary fixture impact unless the live test bodies start including
  Project attachments.
