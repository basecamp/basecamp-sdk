package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for MessageTypes operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class MessageTypesService(client: AccountClient) : BaseService(client) {

    /**
     * List message types in a project
     * @param bucketId The bucket ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(bucketId: Long, options: PaginationOptions? = null): ListResult<MessageType> {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "ListMessageTypes",
            resourceType = "message_type",
            isMutation = false,
            projectId = bucketId,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${bucketId}/categories.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<MessageType>>(body)
        }
    }

    /**
     * Create a new message type in a project
     * @param bucketId The bucket ID
     * @param body Request body
     */
    suspend fun create(bucketId: Long, body: CreateMessageTypeBody): MessageType {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "CreateMessageType",
            resourceType = "message_type",
            isMutation = true,
            projectId = bucketId,
            resourceId = null,
        )
        return request(info, {
            httpPost("/buckets/${bucketId}/categories.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
                put("icon", kotlinx.serialization.json.JsonPrimitive(body.icon))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<MessageType>(body)
        }
    }

    /**
     * Get a single message type by id
     * @param bucketId The bucket ID
     * @param typeId The type ID
     */
    suspend fun get(bucketId: Long, typeId: Long): MessageType {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "GetMessageType",
            resourceType = "message_type",
            isMutation = false,
            projectId = bucketId,
            resourceId = typeId,
        )
        return request(info, {
            httpGet("/buckets/${bucketId}/categories/${typeId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<MessageType>(body)
        }
    }

    /**
     * Update an existing message type
     * @param bucketId The bucket ID
     * @param typeId The type ID
     * @param body Request body
     */
    suspend fun update(bucketId: Long, typeId: Long, body: UpdateMessageTypeBody): MessageType {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "UpdateMessageType",
            resourceType = "message_type",
            isMutation = true,
            projectId = bucketId,
            resourceId = typeId,
        )
        return request(info, {
            httpPut("/buckets/${bucketId}/categories/${typeId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.name?.let { put("name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.icon?.let { put("icon", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<MessageType>(body)
        }
    }

    /**
     * Delete a message type
     * @param bucketId The bucket ID
     * @param typeId The type ID
     */
    suspend fun delete(bucketId: Long, typeId: Long): Unit {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "DeleteMessageType",
            resourceType = "message_type",
            isMutation = true,
            projectId = bucketId,
            resourceId = typeId,
        )
        request(info, {
            httpDelete("/buckets/${bucketId}/categories/${typeId}", operationName = info.operation)
        }) { Unit }
    }
}
