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
    suspend fun list(recordingId: Long, options: PaginationOptions? = null): ListResult<ClientReply> {
        val info = OperationInfo(
            service = "ClientReplies",
            operation = "ListClientReplies",
            resourceType = "client_reply",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        return requestPaginated(info, options, {
            httpGet("/client/recordings/${recordingId}/replies.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<ClientReply>>(body)
        }
    }

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
            resourceId = recordingId,
        )
        return request(info, {
            httpGet("/client/recordings/${recordingId}/replies/${replyId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<ClientReply>(body)
        }
    }
}
