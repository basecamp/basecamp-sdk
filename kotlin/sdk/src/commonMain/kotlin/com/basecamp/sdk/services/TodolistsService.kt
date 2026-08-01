package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.services.UpdateTodolistBody
import com.basecamp.sdk.generated.services.UpdateTodolistOrGroupBody
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonObject

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
    suspend fun update(id: Long, body: UpdateTodolistBody): JsonElement {
        val fields = fieldsFromTodolist(get(id))
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
    suspend fun edit(id: Long, block: TodolistFields.() -> Unit): JsonElement {
        val fields = fieldsFromTodolist(get(id))
        fields.block()
        return putFields(id, fields)
    }

    /**
     * Reads the writable state out of a fetched todolist.
     *
     * The route is polymorphic — the same URI addresses a todolist or a
     * todolist group — and BC3 renders both through the same jbuilder, so the
     * two projections are wire-identical and this reads them identically, with
     * no type sniffing. The wire carries the recordable's FLAT JSON; the
     * `{"todolist": ...}` / `{"group": ...}` envelope in the Smithy spec is a
     * modelling convention (see AGENTS.md, "Smithy Spec vs Actual API
     * Responses"), tolerated here only because unwrapping it costs one lookup.
     */
    private fun fieldsFromTodolist(todolist: JsonElement): TodolistFields {
        val root = todolist.jsonObject
        val obj = root["todolist"] as? JsonObject ?: root["group"] as? JsonObject ?: root
        return TodolistFields(
            name = obj.string("name"),
            description = obj.string("description"),
        )
    }

    /** Reads a string field, treating a missing, null, or non-scalar one as empty. */
    private fun JsonObject.string(key: String): String =
        (this[key] as? JsonPrimitive)?.contentOrNull ?: ""

    /**
     * PUTs the full writable state via `replace`: name and description are
     * always sent, the empty description included, so a clear survives the
     * PUT. `""` is how a clear is expressed — never JSON null, which body
     * compaction (SPEC §18) forbids and the generated body builder would drop
     * anyway, turning the clear into an omission the caller never asked for.
     */
    private suspend fun putFields(id: Long, fields: TodolistFields): JsonElement {
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
