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
    suspend fun listReplies(forwardId: Long, options: ListForwardRepliesOptions): ListResult<ForwardReply> {
        val info = OperationInfo(
            service = "Forwards",
            operation = "ListForwardReplies",
            resourceType = "forward_reply",
            isMutation = false,
            projectId = null,
            resourceId = forwardId,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/inbox_forwards/${forwardId}/replies.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<ForwardReply>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [ListForwardRepliesOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [listReplies] needs an expected type to disambiguate.
     */
    suspend fun listReplies(forwardId: Long, options: PaginationOptions? = null): ListResult<ForwardReply> =
        listReplies(forwardId, ListForwardRepliesOptions(maxItems = options?.maxItems))

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
            resourceId = replyId,
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
    suspend fun list(inboxId: Long, options: ListForwardsOptions? = null): ListResult<Forward> {
        val info = OperationInfo(
            service = "Forwards",
            operation = "ListForwards",
            resourceType = "forward",
            isMutation = false,
            projectId = null,
            resourceId = inboxId,
        )
        val qs = buildQueryString(
            "sort" to options?.sort,
            "direction" to options?.direction,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/inboxes/${inboxId}/inbox_forwards.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Forward>>(body)
        }
    }
}
