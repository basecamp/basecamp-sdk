package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Forwards operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class ForwardsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a forward by ID
     * @param forwardId The forward ID
     */
    suspend fun get(forwardId: Long): Forward {
        val info = OperationInfo(
            service = "Forwards",
            operation = "GetForward",
            resourceType = "forward",
            isMutation = false,
            projectId = null,
            resourceId = forwardId,
        )
        return request(info, {
            httpGet("/inbox_forwards/${forwardId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Forward>(body)
        }
    }

    /**
     * List all replies to a forward
     * @param forwardId The forward ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listReplies(forwardId: Long, options: PaginationOptions? = null): ListResult<ForwardReply> {
        val info = OperationInfo(
            service = "Forwards",
            operation = "ListForwardReplies",
            resourceType = "forward_reply",
            isMutation = false,
            projectId = null,
            resourceId = forwardId,
        )
        return requestPaginated(info, options, {
            httpGet("/inbox_forwards/${forwardId}/replies.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<ForwardReply>>(body)
        }
    }

    /**
     * Create a reply to a forward
     * @param forwardId The forward ID
     * @param body Request body
     */
    suspend fun createReply(forwardId: Long, body: CreateForwardReplyBody): ForwardReply {
        val info = OperationInfo(
            service = "Forwards",
            operation = "CreateForwardReply",
            resourceType = "forward_reply",
            isMutation = true,
            projectId = null,
            resourceId = forwardId,
        )
        return request(info, {
            httpPost("/inbox_forwards/${forwardId}/replies.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<ForwardReply>(body)
        }
    }

    /**
     * Get a forward reply by ID
     * @param forwardId The forward ID
     * @param replyId The reply ID
     */
    suspend fun getReply(forwardId: Long, replyId: Long): ForwardReply {
        val info = OperationInfo(
            service = "Forwards",
            operation = "GetForwardReply",
            resourceType = "forward_reply",
            isMutation = false,
            projectId = null,
            resourceId = forwardId,
        )
        return request(info, {
            httpGet("/inbox_forwards/${forwardId}/replies/${replyId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<ForwardReply>(body)
        }
    }

    /**
     * Get an inbox by ID
     * @param inboxId The inbox ID
     */
    suspend fun getInbox(inboxId: Long): Inbox {
        val info = OperationInfo(
            service = "Forwards",
            operation = "GetInbox",
            resourceType = "inbox",
            isMutation = false,
            projectId = null,
            resourceId = inboxId,
        )
        return request(info, {
            httpGet("/inboxes/${inboxId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Inbox>(body)
        }
    }

    /**
     * List all forwards in an inbox
     * @param inboxId The inbox ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(inboxId: Long, options: PaginationOptions? = null): ListResult<Forward> {
        val info = OperationInfo(
            service = "Forwards",
            operation = "ListForwards",
            resourceType = "forward",
            isMutation = false,
            projectId = null,
            resourceId = inboxId,
        )
        return requestPaginated(info, options, {
            httpGet("/inboxes/${inboxId}/forwards.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Forward>>(body)
        }
    }
}
