package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Todolists operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class TodolistsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a single todolist or todolist group by id
     * @param projectId The project ID
     * @param id The id
     */
    suspend fun get(projectId: Long, id: Long): JsonElement {
        val info = OperationInfo(
            service = "Todolists",
            operation = "GetTodolistOrGroup",
            resourceType = "todolist_or_group",
            isMutation = false,
            projectId = projectId,
            resourceId = null,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/todolists/${id}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Update an existing todolist or todolist group
     * @param projectId The project ID
     * @param id The id
     * @param body Request body
     */
    suspend fun update(projectId: Long, id: Long, body: UpdateTodolistOrGroupBody): JsonElement {
        val info = OperationInfo(
            service = "Todolists",
            operation = "UpdateTodolistOrGroup",
            resourceType = "todolist_or_group",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/todolists/${id}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.name?.let { put("name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * List todolists in a todoset
     * @param projectId The project ID
     * @param todosetId The todoset ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(projectId: Long, todosetId: Long, options: ListTodolistsOptions? = null): ListResult<Todolist> {
        val info = OperationInfo(
            service = "Todolists",
            operation = "ListTodolists",
            resourceType = "todolist",
            isMutation = false,
            projectId = projectId,
            resourceId = todosetId,
        )
        val qs = buildQueryString(
            "status" to options?.status,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/buckets/${projectId}/todosets/${todosetId}/todolists.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Todolist>>(body)
        }
    }

    /**
     * Create a new todolist in a todoset
     * @param projectId The project ID
     * @param todosetId The todoset ID
     * @param body Request body
     */
    suspend fun create(projectId: Long, todosetId: Long, body: CreateTodolistBody): Todolist {
        val info = OperationInfo(
            service = "Todolists",
            operation = "CreateTodolist",
            resourceType = "todolist",
            isMutation = true,
            projectId = projectId,
            resourceId = todosetId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/todosets/${todosetId}/todolists.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Todolist>(body)
        }
    }
}
