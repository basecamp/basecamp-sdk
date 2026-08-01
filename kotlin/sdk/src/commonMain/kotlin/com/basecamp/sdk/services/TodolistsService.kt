package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.services.UpdateTodolistBody
import com.basecamp.sdk.generated.services.UpdateTodolistOrGroupBody
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
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
        val root = requireJsonObject(todolist)
        val obj = root["todolist"] as? JsonObject ?: root["group"] as? JsonObject ?: root
        return TodolistFields(
            name = obj.writableString("name", required = true),
            description = obj.writableString("description"),
        )
    }

    /**
     * Reads a writable string field out of the fetched body.
     *
     * An absent key or an explicit JSON null is genuinely empty — there is
     * nothing to preserve, and `""` is what the server already holds.
     * Anything else that is not a JSON string is a malformed response and must
     * NOT be coerced. `contentOrNull` would render a number or boolean as
     * text, and a JSON array or object is not a [JsonPrimitive] at all so it
     * would fall through to `""`. Either way [putFields] would then write that
     * coerced-or-empty value back in the full-replace PUT — silently
     * overwriting or erasing the exact field these composites exist to
     * preserve. Fail before the PUT instead, so a malformed response surfaces
     * as an error rather than as data loss.
     *
     * This path only exists because `GetTodolistOrGroup` is modelled as a
     * `oneOf`, so the Kotlin generator returns an untyped [JsonElement] for it.
     * Every other Kotlin composite reads a typed model, where the decoder
     * already rejects a wrong-typed field. Removing that asymmetry is #544.
     */
    /**
     * The response must be a JSON object before any field is read.
     *
     * One level up from the malformed-field guards: a successful GET can return
     * a scalar, an array, or null. `JsonElement.jsonObject` throws a raw
     * `IllegalArgumentException` for those, which is not the SPEC §6 shape — a
     * caller checking for `BasecampException` would miss it and it carries no
     * hint. `JsonElement.toString()` is safe to interpolate: the type is a
     * closed JSON tree, so rendering it cannot run user code or recurse
     * unboundedly, which is why no separate describe-helper is needed here.
     */
    private fun requireJsonObject(body: JsonElement): JsonObject =
        body as? JsonObject
            ?: throw BasecampException.Api(
                BasecampException.truncateMessage(
                    "GetTodolistOrGroup returned ${body::class.simpleName} where a todolist " +
                        "object was expected: $body"
                ),
                hint = "The merge-safe update/edit read this record's fields before rewriting " +
                    "them, so a non-object body cannot be used. Use replace() to write the " +
                    "record deliberately.",
            )

    private fun JsonObject.writableString(key: String, required: Boolean = false): String =
        when (val value = this[key]) {
            null, JsonNull ->
                if (required) throw missingField(key) else ""
            is JsonPrimitive ->
                when {
                    !value.isString -> throw malformedField(key, value)
                    required && value.content.isEmpty() -> throw emptyField(key)
                    else -> value.content
                }
            else -> throw malformedField(key, value)
        }

    private fun malformedField(key: String, value: JsonElement): BasecampException =
        BasecampException.Api(
            BasecampException.truncateMessage("Todolist field '$key' is not a JSON string: $value"),
            hint = "The merge-safe update/edit resend this field verbatim, so a coerced or " +
                "empty value would overwrite the current one. Fix the response, or use " +
                "replace() to write the record deliberately.",
        )

    private fun missingField(key: String): BasecampException =
        BasecampException.Api(
            "Todolist field '$key' is missing from the response",
            hint = "$key is required and presence-validated server-side, so a todolist without " +
                "one is a malformed response, not an empty value to preserve.",
        )

    private fun emptyField(key: String): BasecampException =
        BasecampException.Api(
            "Todolist field '$key' is empty in the response",
            hint = "$key is presence-validated server-side, so an empty one is a malformed " +
                "response. The caller did not ask to clear it.",
        )

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
