---
gap: rich-text-attachments-coverage
status: absorbed-in-sdk
detected: 2026-07-22
sdk_demand: medium
bc3_pr: 9980
# Referenced by shape/member name (grep-navigable in spec/basecamp.smithy)
# rather than line number, which drifts as the spec grows.
smithy_refs:
  - "RichTextAttachment (spec/basecamp.smithy)"
  - "RichTextAttachmentList (spec/basecamp.smithy)"
  - "Todo$description_attachments (spec/basecamp.smithy)"
  - "Todolist$description_attachments (spec/basecamp.smithy)"
  - "Comment$content_attachments (spec/basecamp.smithy)"
  - "Message$content_attachments (spec/basecamp.smithy)"
  - "Document$content_attachments (spec/basecamp.smithy)"
  - "Upload$description_attachments (spec/basecamp.smithy)"
  - "ScheduleEntry$description_attachments (spec/basecamp.smithy)"
  - "Forward$content_attachments (spec/basecamp.smithy)"
  - "ForwardReply$content_attachments (spec/basecamp.smithy)"
  - "Card$description_attachments (spec/basecamp.smithy)"
  - "ClientApproval$content_attachments (spec/basecamp.smithy)"
  - "ClientCorrespondence$content_attachments (spec/basecamp.smithy)"
  - "ClientReply$content_attachments (spec/basecamp.smithy)"
  - "Recording$content_attachments (spec/basecamp.smithy)"
  - "Recording$description_attachments (spec/basecamp.smithy)"
  - "QuestionAnswer$content_attachments (spec/basecamp.smithy)"
  - "SearchResult$content_attachments (spec/basecamp.smithy)"
  - "SearchResult$description_attachments (spec/basecamp.smithy)"
  - "SearchResult$attachments (spec/basecamp.smithy)"
  - "SearchResultAttachment (spec/basecamp.smithy)"
  - "Gauge$description_attachments (spec/basecamp.smithy)"
  - "GaugeNeedle$description_attachments (spec/basecamp.smithy)"
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
    - GetComment
    - GetMessage
    - Search
    - ListRecordings
---

# Rich-text `*_attachments` arrays are now fully modeled

> **Not an additive BC5 gap.** Every rich-text attribute in a BC3/BC4/BC5 API
> response has *always* been paired with a `*_attachments` array of structured
> file metadata (documented in upstream `doc/api/sections/rich_text.md`,
> addressed by bc3 #9980). The gap was SDK-side: the SDK modeled the rich-text
> *strings* but not their companion attachment arrays. #400 shipped the first
> slice (`Todo.description_attachments`); **#408 (closes #405) completes
> coverage across every modeled rich-text decode path** and flips this entry to
> `absorbed-in-sdk`.
> This is absorption, not a provenance sync — the contract was already within
> the current pin, so no repin.

## What's missing

Nothing outstanding on the modeled decode paths. Per
`doc/api/sections/rich_text.md`, a resource's rich-text attribute (`content`,
`description`, …) is accompanied by a `*_attachments` array named after it —
the structured metadata (`id, sgid, filename, content_type, byte_size,
download_url, previewable, preview_url, thumbnail_url`, plus nullable
`width`/`height`) for the downloadable files embedded in that rich text. The
element shape comes from the jbuilder chain `api/recordings/_rich_text` →
`attachments/_attachment` → `blobs/_blob`.

**Absorbed (18 modeled emitters, 20 members):** every rich-text attribute the
SDK decodes now carries its companion array, reusing `RichTextAttachment` +
`RichTextAttachmentList`:

- **Todo** — `description_attachments` (#400).
- **14 concrete resources, `@required` (always emitted, empty when no inline
  files):** Todolist, Comment, Message, Document, Upload, ScheduleEntry,
  Forward, ForwardReply, Card (`description_attachments`), ClientApproval,
  ClientCorrespondence, ClientReply, QuestionAnswer, and **GaugeNeedle**.
- **Gauge — optional, non-nullable:** the type-specific partial renders the
  companion array only when the gauge has needles (`if gauge.any_needles?`), so
  a needle-less gauge omits the key.
- **2 polymorphic projections — optional, non-nullable, both arrays each:**
  **SearchResult** (`searches/show.json.jbuilder` renders the full type-specific
  partial via `api_search_result_template_path`, nulls the strings but keeps the
  arrays) and the generic **Recording** (`projects/recordings/index` renders
  `to_recordable_partial_path`). A given item carries only the array matching its
  type; a webhook-sourced item carries neither.

**Also absorbed — the generic `attachments` key on SearchResult (#716,
`SearchResult$attachments` / `SearchResultAttachment`):** this entry
previously listed the key as explicitly out of scope, "a redundant projection
of the companion array, not a distinct aggregate". (That was this item's
*second* correction: an earlier version called it "the recording's aggregate
downloadable files, a *different* projection concern", and #428 — filed on
that premise — is closed as a result.) The redundancy analysis, verified
against bc3 `c308693171`, was sound for its path — `searches/show.json.jbuilder`
emits `recording.downloadable_attachments` through the same
`attachments/_attachment` partial that builds the companion array, and
`RichText.rich_text_attribute` registers exactly one rich-text attribute per
model, so on THAT path the two keys always carry identical elements — but it
missed a second emitter: for chat *upload* lines,
`chats/lines/_upload.json.jbuilder` builds a bespoke six-key `attachments`
aggregate (`title`, `url`, `filename`, `content_type`, `byte_size`,
`download_url`) inline, and because upload lines have no `rich_text_content`,
`has_downloadable_attachments?` is false and the show template never
overwrites it — the bespoke shape reaches the wire. That shape lacks
`RichTextAttachment`'s required `id`/`sgid`/`previewable`/`preview_url`/
`thumbnail_url`, so #716 models the key as its own element type,
`SearchResultAttachment`: an optional-field superset of both wire variants
with only the four keys both always emit required (re-verified against the
revision `spec/api-provenance.json` currently pins).

**Explicitly out of scope (not "absorbed" — accurate status per item):**
- **everything/aggregates endpoints** (`everything/*`, which also render full
  partials): **unmodeled in the SDK** — no decode path exists to carry an array
  into. Covered by the separate `everything-aggregates.md` gap
  (`addressed-in-bc3-pr-11627`); the companion arrays arrive for free when those
  endpoints are absorbed.
- **Webhook-sourced generic Recording** (`webhooks/event.jbuilder:5` renders the
  **base** `recordings/recording` partial, which emits **no** arrays): handled by
  the optional Recording arrays being absent — not a gap.
- **JournalEntry:** undocumented / out of API scope. Not modeled.
- **google_documents, cloud_files:** unmodeled bc3 resources with their own gap
  entries; their rich-text arrays are covered when those resources are absorbed.
- **clients/forwards, clients/introductions:** `Client::Correspondence`
  sub-forms, not standalone modeled decode paths. Not modeled here.

## Why it matters

Consumers enumerating the files embedded in a recording's rich text (to list,
download, or preview them) can now do so for every modeled rich-text resource,
not just a Todo's `description`. The arrays are emitted by the API on every
read, so the gap was purely in the SDK's typed surface — a read-path
completeness gap, invisible to per-resource schema validation because each
resource validates fine without the (additive) array.

## Suggested API shape

None new — the contract already exists and is stable. Each resource gained a
`*_attachments: RichTextAttachmentList` member reusing the `RichTextAttachment`
structure: `@required` for the 14 concrete always-emitting resources, optional
(no `@required`) for Gauge and the two projections whose partials render the
array conditionally. `width`/`height` stay optional/nullable with the cross-SDK
decode caveat recorded in SPEC.md §10 Type Fidelity.

**Optional-array presence contract.** For the three optional members (Gauge and
the SearchResult/Recording projections) an *absent* array and a *present-but-empty*
one are distinct wire states — absent means the item is not that rich-text type
(a non-matching projection type, or a webhook-sourced item rendered from the base
partial), while present-empty means the matching type carries no inline files. Per
SPEC.md's Optional Fields rule (a sentinel is not a substitute for absence), every
SDK models these as nullable/optional so all three wire states — absent,
present-empty, populated — round-trip faithfully in both directions: Go
`*[]RichTextAttachment` (nil pointer omitted, non-nil pointer to `[]` re-encodes as
`[]`), Swift `[RichTextAttachment]?`, TypeScript `?: …[]`, Python `NotRequired[…]`,
Ruby nil-vs-`[]` (`parse_array`), and Kotlin `List<RichTextAttachment>? = null`
(the model generator emits `List<T>? = null` for *every* optional array, so an
absent array is distinct from a present-but-empty one). The
14 concrete `@required` members are always present, so they stay a plain
non-optional list in each SDK.

## Implementation notes for BC3

None. The JSON contract is documented (`doc/api/sections/rich_text.md`, bc3
#9980) and shipping in production across BC3/BC4/BC5. This is SDK absorption
work only; no upstream change is required, and no provenance repin — the
contract already sits within the current pin.

## SDK absorption plan when this lands

Complete. Shipped in two increments, both reusing `RichTextAttachment`:

1. **#400:** `Todo.description_attachments` — the `RichTextAttachment` structure
   + `RichTextAttachmentList` (`spec/basecamp.smithy`), faithful decode across
   all six SDKs (float-spelled `1024.0` → `1024`, `null` → nil/null), Go
   round-trip (`null` → `"width": null`), and per-SDK decode tests. The nullable
   float-tolerant dimension representation — Go `types.FlexInt`, Kotlin
   `FlexibleIntSerializer` on a nullable `Int?`, Swift `Int32?`, Python
   `int | float`, Ruby nilable — is documented in SPEC.md §10 Type Fidelity.
2. **#408 (closes #405):** `content_attachments` / `description_attachments` on the
   remaining 17 structures (19 members) — 14 concrete `@required`, Gauge and the
   two polymorphic projections (SearchResult, Recording) optional/non-nullable —
   reusing `RichTextAttachment` and the same per-SDK decode assertions unchanged.
   No new dimension-fidelity work was needed. The out-of-scope items above are
   documented rather than absorbed.
