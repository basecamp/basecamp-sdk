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
     * @param projectId The project ID
     * @param cardId The card ID
     */
    suspend fun get(projectId: Long, cardId: Long): Card {
        val info = OperationInfo(
            service = "Cards",
            operation = "GetCard",
            resourceType = "card",
            isMutation = false,
            projectId = projectId,
            resourceId = cardId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/card_tables/cards/${cardId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Card>(body)
        }
    }

    /**
     * Update an existing card
     * @param projectId The project ID
     * @param cardId The card ID
     * @param body Request body
     */
    suspend fun update(projectId: Long, cardId: Long, body: UpdateCardBody): Card {
        val info = OperationInfo(
            service = "Cards",
            operation = "UpdateCard",
            resourceType = "card",
            isMutation = true,
            projectId = projectId,
            resourceId = cardId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/card_tables/cards/${cardId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
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
     * @param projectId The project ID
     * @param cardId The card ID
     * @param body Request body
     */
    suspend fun move(projectId: Long, cardId: Long, body: MoveCardBody): Unit {
        val info = OperationInfo(
            service = "Cards",
            operation = "MoveCard",
            resourceType = "card",
            isMutation = true,
            projectId = projectId,
            resourceId = cardId,
        )
        request(info, {
            httpPost("/buckets/${projectId}/card_tables/cards/${cardId}/moves.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("column_id", kotlinx.serialization.json.JsonPrimitive(body.columnId))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * List cards in a column
     * @param projectId The project ID
     * @param columnId The column ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(projectId: Long, columnId: Long, options: PaginationOptions? = null): ListResult<Card> {
        val info = OperationInfo(
            service = "Cards",
            operation = "ListCards",
            resourceType = "card",
            isMutation = false,
            projectId = projectId,
            resourceId = columnId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/card_tables/lists/${columnId}/cards.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Card>>(body)
        }
    }

    /**
     * Create a card in a column
     * @param projectId The project ID
     * @param columnId The column ID
     * @param body Request body
     */
    suspend fun create(projectId: Long, columnId: Long, body: CreateCardBody): Card {
        val info = OperationInfo(
            service = "Cards",
            operation = "CreateCard",
            resourceType = "card",
            isMutation = true,
            projectId = projectId,
            resourceId = columnId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/card_tables/lists/${columnId}/cards.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
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
