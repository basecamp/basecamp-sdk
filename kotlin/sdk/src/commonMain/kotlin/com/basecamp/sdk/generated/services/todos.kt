package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Todos operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class TodosService(client: AccountClient) : BaseService(client) {

    /**
     * List todos in a todolist
     * @param todolistId The todolist ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(todolistId: Long, options: ListTodosOptions? = null): ListResult<Todo> {
        val info = OperationInfo(
            service = "Todos",
            operation = "ListTodos",
            resourceType = "todo",
            isMutation = false,
            projectId = null,
            resourceId = todolistId,
        )
        val qs = buildQueryString(
            "status" to options?.status,
            "completed" to options?.completed,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/todolists/${todolistId}/todos.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Todo>>(body)
        }
    }

    /**
     * Create a new todo in a todolist
     * @param todolistId The todolist ID
     * @param body Request body
     */
    suspend fun create(todolistId: Long, body: CreateTodoBody): Todo {
        val info = OperationInfo(
            service = "Todos",
            operation = "CreateTodo",
            resourceType = "todo",
            isMutation = true,
            projectId = null,
            resourceId = todolistId,
        )
        return request(info, {
            httpPost("/todolists/${todolistId}/todos.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.assigneeIds?.let { put("assignee_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.completionSubscriberIds?.let { put("completion_subscriber_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.notify?.let { put("notify", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.dueOn?.let { put("due_on", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.startsOn?.let { put("starts_on", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Todo>(body)
        }
    }

    /**
     * Get a single todo by id
     * @param todoId The todo ID
     */
    suspend fun get(todoId: Long): Todo {
        val info = OperationInfo(
            service = "Todos",
            operation = "GetTodo",
            resourceType = "todo",
            isMutation = false,
            projectId = null,
            resourceId = todoId,
        )
        return request(info, {
            httpGet("/todos/${todoId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Todo>(body)
        }
    }

    /**
     * Update an existing todo
     * @param todoId The todo ID
     * @param body Request body
     */
    suspend fun update(todoId: Long, body: UpdateTodoBody): Todo {
        val info = OperationInfo(
            service = "Todos",
            operation = "UpdateTodo",
            resourceType = "todo",
            isMutation = true,
            projectId = null,
            resourceId = todoId,
        )
        return request(info, {
            httpPut("/todos/${todoId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.content?.let { put("content", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.assigneeIds?.let { put("assignee_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.completionSubscriberIds?.let { put("completion_subscriber_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.notify?.let { put("notify", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.dueOn?.let { put("due_on", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.startsOn?.let { put("starts_on", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Todo>(body)
        }
    }

    /**
     * Trash a todo. Trashed items can be recovered.
     * @param todoId The todo ID
     */
    suspend fun trash(todoId: Long): Unit {
        val info = OperationInfo(
            service = "Todos",
            operation = "TrashTodo",
            resourceType = "todo",
            isMutation = true,
            projectId = null,
            resourceId = todoId,
        )
        request(info, {
            httpDelete("/todos/${todoId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Mark a todo as complete
     * @param todoId The todo ID
     */
    suspend fun complete(todoId: Long): Unit {
        val info = OperationInfo(
            service = "Todos",
            operation = "CompleteTodo",
            resourceType = "todo",
            isMutation = true,
            projectId = null,
            resourceId = todoId,
        )
        request(info, {
            httpPost("/todos/${todoId}/completion.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Mark a todo as incomplete
     * @param todoId The todo ID
     */
    suspend fun uncomplete(todoId: Long): Unit {
        val info = OperationInfo(
            service = "Todos",
            operation = "UncompleteTodo",
            resourceType = "todo",
            isMutation = true,
            projectId = null,
            resourceId = todoId,
        )
        request(info, {
            httpDelete("/todos/${todoId}/completion.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Reposition a todo within its todolist
     * @param todoId The todo ID
     * @param body Request body
     */
    suspend fun reposition(todoId: Long, body: RepositionTodoBody): Unit {
        val info = OperationInfo(
            service = "Todos",
            operation = "RepositionTodo",
            resourceType = "todo",
            isMutation = true,
            projectId = null,
            resourceId = todoId,
        )
        request(info, {
            httpPut("/todos/${todoId}/position.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("position", kotlinx.serialization.json.JsonPrimitive(body.position))
                body.parentId?.let { put("parent_id", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { Unit }
    }
}
