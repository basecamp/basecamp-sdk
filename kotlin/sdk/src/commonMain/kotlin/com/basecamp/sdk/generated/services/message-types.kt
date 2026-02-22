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
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(options: PaginationOptions? = null): ListResult<MessageType> {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "ListMessageTypes",
            resourceType = "message_type",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/categories.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<MessageType>>(body)
        }
    }

    /**
     * Create a new message type in a project
     * @param body Request body
     */
    suspend fun create(body: CreateMessageTypeBody): MessageType {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "CreateMessageType",
            resourceType = "message_type",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/categories.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
                put("icon", kotlinx.serialization.json.JsonPrimitive(body.icon))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<MessageType>(body)
        }
    }

    /**
     * Get a single message type by id
     * @param typeId The type ID
     */
    suspend fun get(typeId: Long): MessageType {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "GetMessageType",
            resourceType = "message_type",
            isMutation = false,
            projectId = null,
            resourceId = typeId,
        )
        return request(info, {
            httpGet("/categories/${typeId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<MessageType>(body)
        }
    }

    /**
     * Update an existing message type
     * @param typeId The type ID
     * @param body Request body
     */
    suspend fun update(typeId: Long, body: UpdateMessageTypeBody): MessageType {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "UpdateMessageType",
            resourceType = "message_type",
            isMutation = true,
            projectId = null,
            resourceId = typeId,
        )
        return request(info, {
            httpPut("/categories/${typeId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.name?.let { put("name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.icon?.let { put("icon", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<MessageType>(body)
        }
    }

    /**
     * Delete a message type
     * @param typeId The type ID
     */
    suspend fun delete(typeId: Long): Unit {
        val info = OperationInfo(
            service = "MessageTypes",
            operation = "DeleteMessageType",
            resourceType = "message_type",
            isMutation = true,
            projectId = null,
            resourceId = typeId,
        )
        request(info, {
            httpDelete("/categories/${typeId}", operationName = info.operation)
        }) { Unit }
    }
}
