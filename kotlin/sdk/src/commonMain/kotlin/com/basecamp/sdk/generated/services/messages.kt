package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Messages operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class MessagesService(client: AccountClient) : BaseService(client) {

    /**
     * List messages on a message board
     * @param projectId The project ID
     * @param boardId The board ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(projectId: Long, boardId: Long, options: PaginationOptions? = null): ListResult<Message> {
        val info = OperationInfo(
            service = "Messages",
            operation = "ListMessages",
            resourceType = "message",
            isMutation = false,
            projectId = projectId,
            resourceId = boardId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/message_boards/${boardId}/messages.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Message>>(body)
        }
    }

    /**
     * Create a new message on a message board
     * @param projectId The project ID
     * @param boardId The board ID
     * @param body Request body
     */
    suspend fun create(projectId: Long, boardId: Long, body: CreateMessageBody): Message {
        val info = OperationInfo(
            service = "Messages",
            operation = "CreateMessage",
            resourceType = "message",
            isMutation = true,
            projectId = projectId,
            resourceId = boardId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/message_boards/${boardId}/messages.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("subject", kotlinx.serialization.json.JsonPrimitive(body.subject))
                body.content?.let { put("content", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.status?.let { put("status", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.categoryId?.let { put("category_id", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Message>(body)
        }
    }

    /**
     * Get a single message by id
     * @param projectId The project ID
     * @param messageId The message ID
     */
    suspend fun get(projectId: Long, messageId: Long): Message {
        val info = OperationInfo(
            service = "Messages",
            operation = "GetMessage",
            resourceType = "message",
            isMutation = false,
            projectId = projectId,
            resourceId = messageId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/messages/${messageId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Message>(body)
        }
    }

    /**
     * Update an existing message
     * @param projectId The project ID
     * @param messageId The message ID
     * @param body Request body
     */
    suspend fun update(projectId: Long, messageId: Long, body: UpdateMessageBody): Message {
        val info = OperationInfo(
            service = "Messages",
            operation = "UpdateMessage",
            resourceType = "message",
            isMutation = true,
            projectId = projectId,
            resourceId = messageId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/messages/${messageId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.subject?.let { put("subject", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.content?.let { put("content", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.status?.let { put("status", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.categoryId?.let { put("category_id", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Message>(body)
        }
    }

    /**
     * Pin a message to the top of the message board
     * @param projectId The project ID
     * @param messageId The message ID
     */
    suspend fun pin(projectId: Long, messageId: Long): Unit {
        val info = OperationInfo(
            service = "Messages",
            operation = "PinMessage",
            resourceType = "message",
            isMutation = true,
            projectId = projectId,
            resourceId = messageId,
        )
        request(info, {
            httpPost("/buckets/${projectId}/recordings/${messageId}/pin.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Unpin a message from the message board
     * @param projectId The project ID
     * @param messageId The message ID
     */
    suspend fun unpin(projectId: Long, messageId: Long): Unit {
        val info = OperationInfo(
            service = "Messages",
            operation = "UnpinMessage",
            resourceType = "message",
            isMutation = true,
            projectId = projectId,
            resourceId = messageId,
        )
        request(info, {
            httpDelete("/buckets/${projectId}/recordings/${messageId}/pin.json", operationName = info.operation)
        }) { Unit }
    }
}
