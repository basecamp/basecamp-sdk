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
     * @param bucketId The bucket ID
     * @param recordingId The recording ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(bucketId: Long, recordingId: Long, options: ListClientRepliesOptions): ListResult<ClientReply> {
        val info = OperationInfo(
            service = "ClientReplies",
            operation = "ListClientReplies",
            resourceType = "client_reply",
            isMutation = false,
            projectId = bucketId,
            resourceId = recordingId,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/buckets/${bucketId}/client/recordings/${recordingId}/replies.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<ClientReply>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [ListClientRepliesOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [list] needs an expected type to disambiguate.
     */
    suspend fun list(bucketId: Long, recordingId: Long, options: PaginationOptions? = null): ListResult<ClientReply> =
        list(bucketId, recordingId, ListClientRepliesOptions(maxItems = options?.maxItems))

    /**
     * Get a single client reply by id
     * @param bucketId The bucket ID
     * @param recordingId The recording ID
     * @param replyId The reply ID
     */
    suspend fun get(bucketId: Long, recordingId: Long, replyId: Long): ClientReply {
        val info = OperationInfo(
            service = "ClientReplies",
            operation = "GetClientReply",
            resourceType = "client_reply",
            isMutation = false,
            projectId = bucketId,
            resourceId = replyId,
        )
        return request(info, {
            httpGet("/buckets/${bucketId}/client/recordings/${recordingId}/replies/${replyId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<ClientReply>(body)
        }
    }
}
