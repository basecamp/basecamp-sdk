package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Doors operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class DoorsService(client: AccountClient) : BaseService(client) {

    /**
     * List the active external links in a project
     * @param bucketId The bucket ID
     */
    suspend fun list(bucketId: Long): List<Door> {
        val info = OperationInfo(
            service = "Doors",
            operation = "ListDoors",
            resourceType = "door",
            isMutation = false,
            projectId = bucketId,
            resourceId = null,
        )
        return request(info, {
            httpGet("/buckets/${bucketId}/dock/doors.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Door>>(body)
        }
    }

    /**
     * Create an external link in a project
     * @param bucketId The bucket ID
     * @param body Request body
     */
    suspend fun create(bucketId: Long, body: CreateDoorBody): Door {
        val info = OperationInfo(
            service = "Doors",
            operation = "CreateDoor",
            resourceType = "door",
            isMutation = true,
            projectId = bucketId,
            resourceId = null,
        )
        return request(info, {
            httpPost("/buckets/${bucketId}/dock/doors.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("door", body.door)
                body.visibleToClients?.let { put("visible_to_clients", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Door>(body)
        }
    }

    /**
     * Get an external link by id
     * @param doorId The door ID
     */
    suspend fun get(doorId: Long): Door {
        val info = OperationInfo(
            service = "Doors",
            operation = "GetDoor",
            resourceType = "door",
            isMutation = false,
            projectId = null,
            resourceId = doorId,
        )
        return request(info, {
            httpGet("/dock/doors/${doorId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Door>(body)
        }
    }

    /**
     * Update an external link
     * @param doorId The door ID
     * @param body Request body
     */
    suspend fun update(doorId: Long, body: UpdateDoorBody): Door {
        val info = OperationInfo(
            service = "Doors",
            operation = "UpdateDoor",
            resourceType = "door",
            isMutation = true,
            projectId = null,
            resourceId = doorId,
        )
        return request(info, {
            httpPut("/dock/doors/${doorId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.door?.let { put("door", it) }
                body.visibleToClients?.let { put("visible_to_clients", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Door>(body)
        }
    }

    /**
     * Delete an external link
     * @param doorId The door ID
     */
    suspend fun delete(doorId: Long): Unit {
        val info = OperationInfo(
            service = "Doors",
            operation = "DeleteDoor",
            resourceType = "door",
            isMutation = true,
            projectId = null,
            resourceId = doorId,
        )
        request(info, {
            httpDelete("/dock/doors/${doorId}", operationName = info.operation)
        }) { Unit }
    }
}
