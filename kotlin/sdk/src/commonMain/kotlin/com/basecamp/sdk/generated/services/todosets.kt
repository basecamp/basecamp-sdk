package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Todosets operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class TodosetsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a todoset (container for todolists in a project)
     * @param todosetId The todoset ID
     */
    suspend fun get(todosetId: Long): Todoset {
        val info = OperationInfo(
            service = "Todosets",
            operation = "GetTodoset",
            resourceType = "todoset",
            isMutation = false,
            projectId = null,
            resourceId = todosetId,
        )
        return request(info, {
            httpGet("/todosets/${todosetId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Todoset>(body)
        }
    }
}
