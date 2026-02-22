package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for MessageBoards operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class MessageBoardsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a message board
     * @param boardId The board ID
     */
    suspend fun get(boardId: Long): MessageBoard {
        val info = OperationInfo(
            service = "MessageBoards",
            operation = "GetMessageBoard",
            resourceType = "message_board",
            isMutation = false,
            projectId = null,
            resourceId = boardId,
        )
        return request(info, {
            httpGet("/message_boards/${boardId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<MessageBoard>(body)
        }
    }
}
