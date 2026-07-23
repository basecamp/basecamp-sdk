---
gap: visible-to-clients-on-creates
status: addressed-in-bc3-pr-12382
detected: 2026-07-22
sdk_demand: medium
# #12382 is the originating PR (six content creates); #12386 folded the same
# create-time visible_to_clients into dock tool create (CreateTool).
bc3_pr: "12382, 12386"
bc3_refs:
  routes:
    - POST /:account_id/vaults/:vault_id/documents.json
    - POST /:account_id/message_boards/:board_id/messages.json
    - POST /:account_id/questionnaires/:questionnaire_id/questions.json
    - POST /:account_id/schedules/:schedule_id/entries.json
    - POST /:account_id/todosets/:todoset_id/todolists.json
    - POST /:account_id/vaults/:vault_id/uploads.json
    - POST /:account_id/buckets/:bucket_id/dock/tools.json
  controllers:
    - app/controllers/documents_controller.rb
    - app/controllers/messages_controller.rb
    - app/controllers/questions_controller.rb
    - app/controllers/schedules/entries_controller.rb
    - app/controllers/todolists_controller.rb
    - app/controllers/uploads_controller.rb
    - app/controllers/docks/tools_controller.rb
  related_existing_api:
    - CreateDocument
    - CreateMessage
    - CreateQuestion
    - CreateScheduleEntry
    - CreateTodolist
    - CreateUpload
    - CreateTool
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
create param to **dock tool creation** (`POST /:account_id/buckets/:bucket_id/dock/tools.json`,
`CreateTool`). It takes effect only for the tool types that manage their own
visibility — `Chat::Transcript` and `Kanban::Board` — which otherwise start
hidden from clients; all other tool types ignore it. `CreateTool` is already
modeled (see [[dock-tool-create-contract]]) but does not carry this field yet.

None of the corresponding Smithy `Create*Input` structures model this field
today:

- `CreateDocumentInput`
- `CreateMessageInput`
- `CreateQuestionInput`
- `CreateScheduleEntryInput`
- `CreateTodolistInput`
- `CreateUploadInput`
- `CreateToolInput` (dock tools, #12386; effective for `Chat::Transcript` /
  `Kanban::Board` only)

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

An additive optional top-level `visible_to_clients` boolean on each of the
seven create request bodies (sibling of the wrapped resource object). Response
shape unchanged.

## Implementation notes for BC3

- Already merged: #12382 (six content creates) + #12386 (dock tool create).
- The param is permitted at the top level of the create request, not inside the
  wrapped `document`/`message`/etc. object — confirm the exact permit location
  per controller before absorption.
- Default when omitted (content creates): `false` (hidden) for team callers; a
  client caller always creates client-visible records; folder-nested vault items
  inherit the folder's visibility. For dock tools it only takes effect for
  `Chat::Transcript` / `Kanban::Board`; other tool types ignore it.

## SDK absorption plan when this lands

- Add an optional top-level `visible_to_clients: Boolean` member to each of the
  seven `Create*Input` structures (the six content creates + `CreateToolInput`).
  Use the snake_case wire name directly as the Smithy member name (matching
  existing body members like `attachable_sgid`); a camelCase `visibleToClients`
  member would emit the wrong JSON key. Confirm per controller whether it is
  permitted at the top level vs. inside the wrapped params object; the docs show
  it as a top-level sibling.
- Regenerate all six SDKs; add create-with-visibility coverage.
- No new operations or tags — all seven creates already exist and are tagged.
- Response shape is unchanged.

## Compatibility

Additive optional param. For the **six content creates** it is silently ignored
on BC4 and honored on BC5 — no invariant violation. **`CreateTool` is the
exception:** its current request shape already fails against `four` with 400
because it cannot emit the BC4-required `source_recording_id` (see
[[dock-tool-create-contract]] — the tool_type contract is BC5-only), so BC4
never reaches the point of silently ignoring `visible_to_clients` on that
operation. The "silently ignored on BC4" statement therefore applies to the six
content creates, not to `CreateTool`.
