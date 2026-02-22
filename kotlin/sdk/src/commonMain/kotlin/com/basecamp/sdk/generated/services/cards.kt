package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Cards operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class CardsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a card by ID
     * @param cardId The card ID
     */
    suspend fun get(cardId: Long): Card {
        val info = OperationInfo(
            service = "Cards",
            operation = "GetCard",
            resourceType = "card",
            isMutation = false,
            projectId = null,
            resourceId = cardId,
        )
        return request(info, {
            httpGet("/card_tables/cards/${cardId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Card>(body)
        }
    }

    /**
     * Update an existing card
     * @param cardId The card ID
     * @param body Request body
     */
    suspend fun update(cardId: Long, body: UpdateCardBody): Card {
        val info = OperationInfo(
            service = "Cards",
            operation = "UpdateCard",
            resourceType = "card",
            isMutation = true,
            projectId = null,
            resourceId = cardId,
        )
        return request(info, {
            httpPut("/card_tables/cards/${cardId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.content?.let { put("content", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.dueOn?.let { put("due_on", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.assigneeIds?.let { put("assignee_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Card>(body)
        }
    }

    /**
     * Move a card to a different column
     * @param cardId The card ID
     * @param body Request body
     */
    suspend fun move(cardId: Long, body: MoveCardBody): Unit {
        val info = OperationInfo(
            service = "Cards",
            operation = "MoveCard",
            resourceType = "card",
            isMutation = true,
            projectId = null,
            resourceId = cardId,
        )
        request(info, {
            httpPost("/card_tables/cards/${cardId}/moves.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("column_id", kotlinx.serialization.json.JsonPrimitive(body.columnId))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * List cards in a column
     * @param columnId The column ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(columnId: Long, options: PaginationOptions? = null): ListResult<Card> {
        val info = OperationInfo(
            service = "Cards",
            operation = "ListCards",
            resourceType = "card",
            isMutation = false,
            projectId = null,
            resourceId = columnId,
        )
        return requestPaginated(info, options, {
            httpGet("/card_tables/lists/${columnId}/cards.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Card>>(body)
        }
    }

    /**
     * Create a card in a column
     * @param columnId The column ID
     * @param body Request body
     */
    suspend fun create(columnId: Long, body: CreateCardBody): Card {
        val info = OperationInfo(
            service = "Cards",
            operation = "CreateCard",
            resourceType = "card",
            isMutation = true,
            projectId = null,
            resourceId = columnId,
        )
        return request(info, {
            httpPost("/card_tables/lists/${columnId}/cards.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
                body.content?.let { put("content", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.dueOn?.let { put("due_on", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.notify?.let { put("notify", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Card>(body)
        }
    }
}
