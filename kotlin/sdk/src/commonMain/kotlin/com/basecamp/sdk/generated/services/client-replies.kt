package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for ClientReplies operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class ClientRepliesService(client: AccountClient) : BaseService(client) {

    /**
     * List all client replies for a recording (correspondence or approval)
     * @param recordingId The recording ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(recordingId: Long, options: ListClientRepliesOptions? = null): ListResult<ClientReply> {
        val info = OperationInfo(
            service = "ClientReplies",
            operation = "ListClientReplies",
            resourceType = "client_reply",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/client/recordings/${recordingId}/replies.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<ClientReply>>(body)
        }
    }

    /**
     * Source-compatibility overload: accepts bare [PaginationOptions].
     *
     * Prefer [ListClientRepliesOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     */
    suspend fun list(recordingId: Long, options: PaginationOptions): ListResult<ClientReply> =
        list(recordingId, ListClientRepliesOptions(maxItems = options.maxItems))

    /**
     * Get a single client reply by id
     * @param recordingId The recording ID
     * @param replyId The reply ID
     */
    suspend fun get(recordingId: Long, replyId: Long): ClientReply {
        val info = OperationInfo(
            service = "ClientReplies",
            operation = "GetClientReply",
            resourceType = "client_reply",
            isMutation = false,
            projectId = null,
            resourceId = replyId,
        )
        return request(info, {
            httpGet("/client/recordings/${recordingId}/replies/${replyId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<ClientReply>(body)
        }
    }
}
