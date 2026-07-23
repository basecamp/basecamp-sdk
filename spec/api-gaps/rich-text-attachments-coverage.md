---
gap: rich-text-attachments-coverage
status: addressed-in-bc3-pr-9980
detected: 2026-07-22
sdk_demand: medium
bc3_pr: 9980
bc3_refs:
  introduced_in: three
  routes:
    - "GET /:account_id/buckets/:bucket_id/todos/:id.json"
  controllers:
    - app/views/api/recordings/_rich_text.json.jbuilder
    - app/views/api/attachments/_attachment.json.jbuilder
    - app/views/api/blobs/_blob.json.jbuilder
  related_existing_api:
    - GetTodo
---

# Rich-text `*_attachments` arrays are only partially modeled

> **Not an additive BC5 gap.** Every rich-text attribute in a BC3/BC4/BC5 API
> response has *always* been paired with a `*_attachments` array of structured
> file metadata (documented in upstream `doc/api/sections/rich_text.md`,
> addressed by bc3 #9980). The gap is SDK-side: the SDK modeled the rich-text
> *strings* but not their companion attachment arrays. This entry records the
> first slice of coverage (`Todo.description_attachments`) and the residual.

## What's missing

Per `doc/api/sections/rich_text.md`, a resource's rich-text attribute
(`content`, `description`, …) is accompanied by a `*_attachments` array named
after it — the structured metadata (`id, sgid, filename, content_type,
byte_size, download_url, previewable, preview_url, thumbnail_url`, plus
nullable `width`/`height`) for the downloadable files embedded in that rich
text. The array is always present (empty when the rich text has no inline
files). The element shape comes from the jbuilder chain
`api/recordings/_rich_text` → `attachments/_attachment` → `blobs/_blob`.

**Absorbed by this PR:** `Todo.description_attachments` (the field
`basecamp/basecamp-cli#449` observed missing from `todos show --json`), modeled
as `RichTextAttachment` + `RichTextAttachmentList`.

**Residual — still unmodeled:** the companion `content_attachments` /
`description_attachments` arrays on the other ten rich-text resources, all
confirmed lacking a modeled array today:

- Comment (`content_attachments`)
- ClientApproval, ClientCorrespondence, ClientReply (`content_attachments`)
- Document (`content_attachments`)
- Message (`content_attachments`)
- QuestionAnswer (`content_attachments`)
- ScheduleEntry (`description_attachments` — its rich-text attribute is `description`)
- Todolist (`description_attachments`)
- Upload (`description_attachments`)

## Why it matters

Consumers enumerating the files embedded in a recording's rich text (to list,
download, or preview them) can do so for a Todo's `description` but not for any
other rich-text resource. The arrays are emitted by the API on every read, so
the gap is purely in the SDK's typed surface — a read-path completeness gap,
invisible to per-resource schema validation because each resource validates
fine without the (additive) array.

## Suggested API shape

None new — the contract already exists and is stable. Each residual resource
gains a `*_attachments: RichTextAttachmentList` member (`@required`, always
emitted, empty when no inline files) reusing the `RichTextAttachment` structure
introduced here. `width`/`height` stay optional/nullable with the cross-SDK
decode caveat recorded in SPEC.md §10 Type Fidelity.

## Implementation notes for BC3

None. The JSON contract is documented (`doc/api/sections/rich_text.md`, bc3
#9980) and shipping in production across BC3/BC4/BC5. This is SDK absorption
work only; no upstream change is required.

## SDK absorption plan when this lands

Incremental, resource by resource, reusing `RichTextAttachment`:

1. **Done (this PR):** `Todo.description_attachments` — the `RichTextAttachment`
   structure + `RichTextAttachmentList` (`spec/basecamp.smithy`), faithful
   decode across all six SDKs (float-spelled `1024.0` → `1024`,
   `null` → nil/null), Go round-trip (`null` → `"width": null`), and per-SDK
   decode tests. The nullable float-tolerant dimension representation shipped
   here — Go `types.FlexInt`, Kotlin `FlexibleIntSerializer` on a nullable
   `Int?`, Swift `Int32?` — is documented in SPEC.md §10 Type Fidelity.
2. **Follow-up:** add `content_attachments` / `description_attachments` to the
   ten residual resources above, each `@required RichTextAttachmentList`,
   reusing `RichTextAttachment` and the same per-SDK decode assertions. No new
   dimension-fidelity work is needed — the representation from step 1 already
   decodes faithfully everywhere.
