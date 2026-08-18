package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.models.Todolist
import com.basecamp.sdk.generated.services.UpdateTodolistBody
import com.basecamp.sdk.generated.services.UpdateTodolistOrGroupBody
import kotlinx.serialization.SerializationException

/**
 * A todolist's full writable state, the receiver of the
 * [TodolistsService.edit] block. The whole object is PUT back to the server,
 * so clearing a field means setting it empty (`""`) — there is no third
 * state. The writable set is exactly `{name, description}`.
 */
class TodolistFields internal constructor(
    /** The list's name (required; the server rejects an empty one). */
    var name: String,
    /** Rich text description (HTML). Set `""` to clear. */
    var description: String,
)

/**
 * Todolists service with merge-safe [update] and read-modify-write [edit] on
 * top of the generated surface (`get`, `replace`, ...).
 *
 * The endpoint is full-replace: BC3's `TodolistsController#update` rebuilds
 * the recordable from only the permitted params, so a sparse PUT that omits
 * `description` erases it. The raw, destructive-by-design path stays reachable
 * as `replace`.
 *
 * The route is polymorphic — the same URI addresses a to-do list or a to-do
 * list *group* — but there is no second shape to read. BC3 has no group model:
 * `todolists/groups/{index,show}.json.jbuilder` render
 * `todolists/_todolist.json.jbuilder`, so a group is a [Todolist] whose parent
 * is a Todolist, reports `"type": "Todolist"`, and carries
 * `description`/`description_attachments` like any list. The structural
 * discriminator is `group_position_url` in place of `groups_url` — never the
 * type string, which is identical for both. These composites therefore read
 * one decoded [Todolist] with no variant branching at all.
 *
 * Both composites call the public `get` and `replace` methods, so hooks
 * observe the two wire operations (`GetTodolistOrGroup` then
 * `UpdateTodolistOrGroup`), not a synthetic composite.
 *
 * Neither is atomic: there is no conditional-update signal on this endpoint,
 * so a concurrent write between the GET and PUT is overwritten — last write
 * wins for the whole representation. The window is one round-trip. Use
 * `replace` to overwrite deliberately.
 */
class TodolistsService(client: AccountClient) :
    com.basecamp.sdk.generated.services.TodolistsService(client) {

    /**
     * Sets the given fields on a todolist and preserves everything else: GETs
     * the current list, overlays the explicitly-set (non-null) body fields,
     * and PUTs the full representation back. A null field is untouched,
     * guaranteed; an explicitly-passed `""` clears.
     *
     * Not atomic — see the class docs for the GET→PUT race.
     */
    suspend fun update(id: Long, body: UpdateTodolistBody): Todolist {
        val fields = fieldsFromTodolist(fetchTodolist(id))
        body.name?.let { fields.name = it }
        body.description?.let { fields.description = it }
        return putFields(id, fields)
    }

    /**
     * Applies a read-modify-write block to a todolist: GETs the current list,
     * runs the block with the full writable state ([TodolistFields]) as
     * receiver, and PUTs the whole thing back. Clearing a field means setting
     * it empty (`""`) — an untouched field keeps its current value. If the
     * block throws, the edit aborts and nothing is written.
     *
     * ```kotlin
     * account.todolists.edit(todolistId) {
     *     name = "🚨 $name"
     *     description = "" // clearing = setting empty on a full object
     * }
     * ```
     *
     * Not atomic — see the class docs for the GET→PUT race.
     */
    suspend fun edit(id: Long, block: TodolistFields.() -> Unit): Todolist {
        val fields = fieldsFromTodolist(fetchTodolist(id))
        fields.block()
        return putFields(id, fields)
    }

    /**
     * GETs the todolist the composites read their writable state from,
     * normalizing a decode failure into the SPEC §6 shape.
     *
     * kotlinx.serialization is the typed guard the dynamic SDKs write by hand,
     * and it rejects a structurally wrong-typed or missing required field
     * before this composite ever sees it. `BaseService` now renders that as the
     * SPEC §6 malformed-2xx-body shape for every operation (#604) — a
     * statusless, non-retryable [BasecampException.Api] carrying the
     * [SerializationException] as its `cause`. What the base layer cannot know
     * is this composite's own account of the failure: which record failed to
     * decode, and the escape hatch for writing it deliberately. That is what is
     * restated here.
     *
     * The restatement is keyed off [BasecampException.Api.decodeFailure], the
     * slot the base layer's decoder wrapper alone fills, so any other
     * [BasecampException.Api] passes through untouched — and it carries that
     * slot forward through the internal factory, because a restatement of a
     * malformed body is still a malformed body. Rebuilding through the public
     * constructor would drop the marker and tell everything downstream this was
     * not a decode failure (#750). Reading `cause is
     * SerializationException` instead would catch more than this GET's decode:
     * an auth strategy that classifies its own JSON failure that way has its
     * exception propagated untouched by `BasecampHttpClient`, and would arrive
     * here relabelled as a malformed todolist — for a request that was never
     * sent (#730).
     *
     * (Bare JSON scalars are refused as well as structural mismatches. They
     * were not until #598 removed the client-wide `isLenient`, which rendered a
     * number or boolean as a String before this composite could ever see it.)
     */
    private suspend fun fetchTodolist(id: Long): Todolist =
        try {
            get(id)
        } catch (e: BasecampException.Api) {
            val decodeFailure = e.decodeFailure ?: throw e
            throw BasecampException.Api.malformedBody(
                message = "GetTodolistOrGroup returned a body that does not decode as a " +
                    "todolist: ${decodeFailure.message}",
                hint = "The merge-safe update/edit resend this record's fields verbatim, so a " +
                    "malformed response cannot be written back safely. Use replace to write the " +
                    "record deliberately.",
                decodeFailure = decodeFailure,
            )
        }

    /**
     * Reads the writable state out of a fetched todolist.
     *
     * `description` needs no check: it is non-nullable and required on the
     * model, so the decoder has already refused an absent, null or structurally
     * wrong-typed one. BC3's `format_api_content` renders a blank rich text as
     * `""`, never JSON null, so an empty description is a real value to
     * preserve — not a malformed response.
     *
     * `name` is the exception, and it needs a hand-written check the decoder
     * cannot supply. The field is non-nullable on the model, so absent and null
     * are already refused — but `""` decodes fine, and BC3 presence-validates
     * the name, so it can never render one empty. An empty name on a 2xx read
     * is therefore a malformed response, and carrying it into the full-replace
     * PUT would blank the real name on a call that only touched `description`.
     */
    private fun fieldsFromTodolist(todolist: Todolist): TodolistFields {
        if (todolist.name.isEmpty()) {
            throw BasecampException.Api(
                message = "GetTodolistOrGroup returned a todolist with an empty \"name\", " +
                    "but the API never renders it empty",
                hint = "name is presence-validated server-side, so an empty one is a malformed " +
                    "response, not an empty value to preserve. The merge-safe update/edit resend " +
                    "this field verbatim, so it would blank the current one. Use replace to " +
                    "write the record deliberately.",
                retryable = false,
            )
        }
        return TodolistFields(name = todolist.name, description = todolist.description)
    }

    /**
     * PUTs the full writable state via `replace`: name and description are
     * always sent, the empty description included, so a clear survives the
     * PUT. `""` is how a clear is expressed — never JSON null, which body
     * compaction (SPEC §18) forbids and the generated body builder would drop
     * anyway, turning the clear into an omission the caller never asked for.
     */
    private suspend fun putFields(id: Long, fields: TodolistFields): Todolist {
        if (fields.name.isEmpty()) {
            throw BasecampException.Usage(
                "todolist name is required",
                hint = "Set a non-empty name; the server rejects a todolist without one",
            )
        }
        return replace(
            id,
            UpdateTodolistOrGroupBody(
                name = fields.name,
                description = fields.description,
            ),
        )
    }
}
