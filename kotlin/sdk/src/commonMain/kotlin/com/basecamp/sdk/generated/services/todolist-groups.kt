package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for TodolistGroups operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class TodolistGroupsService(client: AccountClient) : BaseService(client) {

    /**
     * Reposition a todolist group
     * @param groupId The group ID
     * @param body Request body
     */
    suspend fun reposition(groupId: Long, body: RepositionTodolistGroupBody): Unit {
        val info = OperationInfo(
            service = "TodolistGroups",
            operation = "RepositionTodolistGroup",
            resourceType = "todolist_group",
            isMutation = true,
            projectId = null,
            resourceId = groupId,
        )
        request(info, {
            httpPut("/todolists/groups/${groupId}/position.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("position", kotlinx.serialization.json.JsonPrimitive(body.position))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * List groups in a todolist
     * @param todolistId The todolist ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(todolistId: Long, options: ListTodolistGroupsOptions): ListResult<Todolist> {
        val info = OperationInfo(
            service = "TodolistGroups",
            operation = "ListTodolistGroups",
            resourceType = "todolist_group",
            isMutation = false,
            projectId = null,
            resourceId = todolistId,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/todolists/${todolistId}/groups.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Todolist>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [ListTodolistGroupsOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [list] needs an expected type to disambiguate.
     */
    suspend fun list(todolistId: Long, options: PaginationOptions? = null): ListResult<Todolist> =
        list(todolistId, ListTodolistGroupsOptions(maxItems = options?.maxItems, page = options?.page))

    /**
     * Create a new group in a todolist
     * @param todolistId The todolist ID
     * @param body Request body
     */
    suspend fun create(todolistId: Long, body: CreateTodolistGroupBody): Todolist {
        val info = OperationInfo(
            service = "TodolistGroups",
            operation = "CreateTodolistGroup",
            resourceType = "todolist_group",
            isMutation = true,
            projectId = null,
            resourceId = todolistId,
        )
        return request(info, {
            httpPost("/todolists/${todolistId}/groups.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Todolist>(body)
        }
    }
}
