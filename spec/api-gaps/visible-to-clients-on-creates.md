---
gap: visible-to-clients-on-creates
status: absorbed-in-sdk
detected: 2026-07-22
sdk_demand: medium
# #12382 documents create-time visible_to_clients on the six content creates,
# absorbed here. The dock-tool (CreateTool) extension (#12386) does NOT carry
# the field yet and is tracked separately — see dock-tool-visible-to-clients.
bc3_pr: 12382
smithy_refs:
  - "CreateDocumentInput.visible_to_clients (spec/basecamp.smithy:2231)"
  - "CreateMessageInput.visible_to_clients (spec/basecamp.smithy:1790)"
  - "CreateQuestionInput.visible_to_clients (spec/basecamp.smithy:6198)"
  - "CreateScheduleEntryInput.visible_to_clients (spec/basecamp.smithy:2627)"
  - "CreateTodolistInput.visible_to_clients (spec/basecamp.smithy:1074)"
  - "CreateUploadInput.visible_to_clients (spec/basecamp.smithy:2352)"
bc3_refs:
  routes:
    - POST /:account_id/vaults/:vault_id/documents.json
    - POST /:account_id/message_boards/:board_id/messages.json
    - POST /:account_id/questionnaires/:questionnaire_id/questions.json
    - POST /:account_id/schedules/:schedule_id/entries.json
    - POST /:account_id/todosets/:todoset_id/todolists.json
    - POST /:account_id/vaults/:vault_id/uploads.json
  controllers:
    - app/controllers/documents_controller.rb
    - app/controllers/messages_controller.rb
    - app/controllers/questions_controller.rb
    - app/controllers/schedules/entries_controller.rb
    - app/controllers/todolists_controller.rb
    - app/controllers/uploads_controller.rb
  related_existing_api:
    - CreateDocument
    - CreateMessage
    - CreateQuestion
    - CreateScheduleEntry
    - CreateTodolist
    - CreateUpload
---

# Create-time `visible_to_clients` on content creates

## What's missing

BC3 **#12382** ("API docs: document create-time `visible_to_clients` on content
creates", merged `fe502d75`, 2026-07-22) documents an additive, optional,
create-time `visible_to_clients` boolean on six already-modeled create
operations: documents, messages, questions, schedule entries, todolists, and
uploads. The param is a top-level sibling of the resource body (not nested
inside the wrapped `document`/`message`/etc. object) and sets client visibility
at creation. When omitted it defaults to `false` (hidden) for **team** callers
creating directly under the docked tool; a **client** caller always creates
client-visible records; and vault items created inside a **folder** inherit the
folder's visibility (the flag applies only when creating directly in the tool's
vault).

BC3 **#12386** ("Honor create-time `visible_to_clients` on dock tool creates",
merged `bee714c74`, 2026-07-23) extends the same top-level `visible_to_clients`
create param to **dock tool creation** (`CreateTool`), effective only for
`Chat::Transcript` / `Kanban::Board`. That extension is **out of scope for this
entry** and is tracked separately in
[[dock-tool-visible-to-clients]] — `CreateToolInput` does not carry the field.

**This entry is now `absorbed-in-sdk`** for the six content creates: each of the
six `Create*Input` structures gained an optional top-level
`visible_to_clients: Boolean` member (see `smithy_refs`), fanned out to all six
SDKs, with tri-state transport coverage (omitted / `true` / explicit `false`
sent-not-dropped):

- `CreateDocumentInput`
- `CreateMessageInput`
- `CreateQuestionInput`
- `CreateScheduleEntryInput`
- `CreateTodolistInput`
- `CreateUploadInput`

The related `cloud_files.md` / `google_documents.md` doc changes (refined
further in #12388, which just shows `visible_to_clients` in the create
examples) only *refine* a `visible_to_clients` param that those endpoints
already documented; those resources (CloudFile / GoogleDocument creates) are
unmodeled in Smithy and tracked under [[recordable-subtypes-doc]].

## Why it matters

Without the field, SDK consumers cannot set client visibility at creation time
and must follow every create with a separate visibility update — two round trips
and a window where content is visible under the wrong policy.

## Suggested API shape

Delivered: an additive optional top-level `visible_to_clients` boolean on each
of the six content-create request bodies (sibling of the wrapped resource
object). Response shape unchanged.

## Implementation notes for BC3

- Already merged: #12382 (six content creates). #12386 (dock tool create) is
  tracked in [[dock-tool-visible-to-clients]].
- The param is permitted at the top level of the create request, not inside the
  wrapped `document`/`message`/etc. object — for the map-bodied question create
  it is a top-level sibling of `title`/`schedule`, never nested in the schedule
  wrapper (a silent server no-op).
- Default when omitted: `false` (hidden) for team callers; a client caller
  always creates client-visible records; folder-nested vault items inherit the
  folder's visibility.

## SDK absorption plan when this lands

Absorbed. Each of the six `Create*Input` structures gained an optional top-level
`visible_to_clients: Boolean` member (snake_case wire name used directly as the
Smithy member name, matching body members like `attachable_sgid`, so the
generated JSON key is correct). The field fans out to all six SDKs via
`make generate`; Go's hand-written create wrappers gained the field and
pass-through, including the map-bodied `CreateQuestion` (top-level key). No new
operations or tags — all six creates already exist and are tagged; response
shape unchanged. Coverage: tri-state transport tests (omitted omits the key /
`true` sent / explicit `false` sent-not-dropped) for all six Go ops plus a
messages body-flow test per other SDK.

## Compatibility

Additive optional param on the six content creates: silently ignored on BC4 and
honored on BC5 — no invariant violation. (The `CreateTool` extension has a
different BC4 story because its request shape is already BC5-only; that is
covered in [[dock-tool-visible-to-clients]] and [[dock-tool-create-contract]].)
