package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.generated.models.Todo
import com.basecamp.sdk.generated.services.ReplaceTodoBody
import com.basecamp.sdk.generated.services.UpdateTodoBody

/**
 * A todo's full writable state, the receiver of the [TodosService.edit]
 * block. The whole object is PUT back to the server, so clearing a field
 * means setting it empty (`""` for strings and dates, `emptyList()` for
 * ID lists) — there is no third state.
 */
class TodoFields internal constructor(
    /** Text content (required; the server rejects an empty one). */
    var content: String,
    /** Rich text description (HTML). Set `""` to clear. */
    var description: String,
    /** Complete list of assigned person IDs. Set `emptyList()` to clear. */
    var assigneeIds: List<Long>,
    /** Complete list of person IDs notified on completion. Set `emptyList()` to clear. */
    var completionSubscriberIds: List<Long>,
    /** Due date (YYYY-MM-DD). Set `""` to clear. */
    var dueOn: String,
    /** Start date (YYYY-MM-DD). Set `""` to clear. */
    var startsOn: String,
    /**
     * Send directive, not todo state: never populated from the current
     * todo and sent only when true, asking the server to notify assignees
     * about this write.
     */
    var notify: Boolean = false,
)

/**
 * Todos service with merge-safe [update] and read-modify-write [edit] on
 * top of the generated surface (`get`, `replace`, ...).
 *
 * Both compose the public `get` and `replace` methods, so hooks observe
 * the two wire operations (`GetTodo` then `ReplaceTodo`), not a synthetic
 * composite.
 *
 * Neither is atomic: there is no conditional-update signal on this
 * endpoint, so a concurrent write between the GET and PUT is overwritten —
 * last write wins for the whole representation. The window is one
 * round-trip. Use `replace` to overwrite deliberately.
 */
class TodosService(client: AccountClient) :
    com.basecamp.sdk.generated.services.TodosService(client) {

    /**
     * Sets the given fields on a todo and preserves everything else: GETs
     * the current todo, overlays the explicitly-set (non-null) body
     * fields, and PUTs the full representation back. A null field is
     * untouched, guaranteed; an explicitly-passed empty list clears.
     *
     * Not atomic — see the class docs for the GET→PUT race.
     */
    suspend fun update(todoId: Long, body: UpdateTodoBody): Todo {
        val fields = fieldsFromTodo(get(todoId))
        body.content?.let { fields.content = it }
        body.description?.let { fields.description = it }
        body.assigneeIds?.let { fields.assigneeIds = it }
        body.completionSubscriberIds?.let { fields.completionSubscriberIds = it }
        body.dueOn?.let { fields.dueOn = it }
        body.startsOn?.let { fields.startsOn = it }
        body.notify?.let { fields.notify = it }
        return putFields(todoId, fields)
    }

    /**
     * Applies a read-modify-write block to a todo: GETs the current todo,
     * runs the block with the full writable state ([TodoFields]) as
     * receiver, and PUTs the whole thing back. Clearing a field means
     * setting it empty (`""` / `emptyList()`) — an untouched field keeps
     * its current value. If the block throws, the edit aborts and nothing
     * is written.
     *
     * ```kotlin
     * account.todos.edit(todoId) {
     *     content = "🚨 $content"
     *     dueOn = "" // clearing = setting empty on a full object
     * }
     * ```
     *
     * Not atomic — see the class docs for the GET→PUT race.
     */
    suspend fun edit(todoId: Long, block: TodoFields.() -> Unit): Todo {
        val fields = fieldsFromTodo(get(todoId))
        fields.block()
        return putFields(todoId, fields)
    }

    private fun fieldsFromTodo(todo: Todo): TodoFields = TodoFields(
        content = todo.content,
        description = todo.description ?: "",
        assigneeIds = todo.assignees.map { it.id },
        completionSubscriberIds = todo.completionSubscribers.map { it.id },
        dueOn = todo.dueOn ?: "",
        startsOn = todo.startsOn ?: "",
    )

    /**
     * PUTs the full writable state via `replace`: content, description,
     * and both ID lists are always sent (empties included, so clears
     * survive); dates only when non-empty (the server clears an omitted
     * date, and `""` is a format error); notify only when true.
     */
    private suspend fun putFields(todoId: Long, fields: TodoFields): Todo = replace(
        todoId,
        ReplaceTodoBody(
            content = fields.content,
            description = fields.description,
            assigneeIds = fields.assigneeIds,
            completionSubscriberIds = fields.completionSubscriberIds,
            dueOn = fields.dueOn.ifEmpty { null },
            startsOn = fields.startsOn.ifEmpty { null },
            notify = if (fields.notify) true else null,
        ),
    )
}
