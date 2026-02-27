package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Comments operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class CommentsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a single comment by id
     * @param commentId The comment ID
     */
    suspend fun get(commentId: Long): Comment {
        val info = OperationInfo(
            service = "Comments",
            operation = "GetComment",
            resourceType = "comment",
            isMutation = false,
            projectId = null,
            resourceId = commentId,
        )
        return request(info, {
            httpGet("/comments/${commentId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Comment>(body)
        }
    }

    /**
     * Update an existing comment
     * @param commentId The comment ID
     * @param body Request body
     */
    suspend fun update(commentId: Long, body: UpdateCommentBody): Comment {
        val info = OperationInfo(
            service = "Comments",
            operation = "UpdateComment",
            resourceType = "comment",
            isMutation = true,
            projectId = null,
            resourceId = commentId,
        )
        return request(info, {
            httpPut("/comments/${commentId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Comment>(body)
        }
    }

    /**
     * List comments on a recording
     * @param recordingId The recording ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(recordingId: Long, options: PaginationOptions? = null): ListResult<Comment> {
        val info = OperationInfo(
            service = "Comments",
            operation = "ListComments",
            resourceType = "comment",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        return requestPaginated(info, options, {
            httpGet("/recordings/${recordingId}/comments.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Comment>>(body)
        }
    }

    /**
     * Create a new comment on a recording
     * @param recordingId The recording ID
     * @param body Request body
     */
    suspend fun create(recordingId: Long, body: CreateCommentBody): Comment {
        val info = OperationInfo(
            service = "Comments",
            operation = "CreateComment",
            resourceType = "comment",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpPost("/recordings/${recordingId}/comments.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Comment>(body)
        }
    }
}
