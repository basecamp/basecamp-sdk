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
     * @param projectId The project ID
     * @param groupId The group ID
     * @param body Request body
     */
    suspend fun reposition(projectId: Long, groupId: Long, body: RepositionTodolistGroupBody): Unit {
        val info = OperationInfo(
            service = "TodolistGroups",
            operation = "RepositionTodolistGroup",
            resourceType = "todolist_group",
            isMutation = true,
            projectId = projectId,
            resourceId = groupId,
        )
        request(info, {
            httpPut("/buckets/${projectId}/todolists/${groupId}/position.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("position", kotlinx.serialization.json.JsonPrimitive(body.position))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * List groups in a todolist
     * @param projectId The project ID
     * @param todolistId The todolist ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(projectId: Long, todolistId: Long, options: PaginationOptions? = null): ListResult<TodolistGroup> {
        val info = OperationInfo(
            service = "TodolistGroups",
            operation = "ListTodolistGroups",
            resourceType = "todolist_group",
            isMutation = false,
            projectId = projectId,
            resourceId = todolistId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/todolists/${todolistId}/groups.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<TodolistGroup>>(body)
        }
    }

    /**
     * Create a new group in a todolist
     * @param projectId The project ID
     * @param todolistId The todolist ID
     * @param body Request body
     */
    suspend fun create(projectId: Long, todolistId: Long, body: CreateTodolistGroupBody): TodolistGroup {
        val info = OperationInfo(
            service = "TodolistGroups",
            operation = "CreateTodolistGroup",
            resourceType = "todolist_group",
            isMutation = true,
            projectId = projectId,
            resourceId = todolistId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/todolists/${todolistId}/groups.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<TodolistGroup>(body)
        }
    }
}
