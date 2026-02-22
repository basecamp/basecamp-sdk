package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for CardColumns operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class CardColumnsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a card column by ID
     * @param columnId The column ID
     */
    suspend fun get(columnId: Long): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "GetCardColumn",
            resourceType = "card_column",
            isMutation = false,
            projectId = null,
            resourceId = columnId,
        )
        return request(info, {
            httpGet("/card_tables/columns/${columnId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Update an existing column
     * @param columnId The column ID
     * @param body Request body
     */
    suspend fun update(columnId: Long, body: UpdateCardColumnBody): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "UpdateCardColumn",
            resourceType = "card_column",
            isMutation = true,
            projectId = null,
            resourceId = columnId,
        )
        return request(info, {
            httpPut("/card_tables/columns/${columnId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Set the color of a column
     * @param columnId The column ID
     * @param body Request body
     */
    suspend fun setColor(columnId: Long, body: SetCardColumnColorBody): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "SetCardColumnColor",
            resourceType = "card_column_color",
            isMutation = true,
            projectId = null,
            resourceId = columnId,
        )
        return request(info, {
            httpPut("/card_tables/columns/${columnId}/color.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("color", kotlinx.serialization.json.JsonPrimitive(body.color))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Enable on-hold section in a column
     * @param columnId The column ID
     */
    suspend fun enableOnHold(columnId: Long): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "EnableCardColumnOnHold",
            resourceType = "card_column_on_hold",
            isMutation = true,
            projectId = null,
            resourceId = columnId,
        )
        return request(info, {
            httpPost("/card_tables/columns/${columnId}/on_hold.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Disable on-hold section in a column
     * @param columnId The column ID
     */
    suspend fun disableOnHold(columnId: Long): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "DisableCardColumnOnHold",
            resourceType = "card_column_on_hold",
            isMutation = true,
            projectId = null,
            resourceId = columnId,
        )
        return request(info, {
            httpDelete("/card_tables/columns/${columnId}/on_hold.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Subscribe to a card column (watch for changes)
     * @param columnId The column ID
     */
    suspend fun subscribeToColumn(columnId: Long): Unit {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "SubscribeToCardColumn",
            resourceType = "to_card_column",
            isMutation = true,
            projectId = null,
            resourceId = columnId,
        )
        request(info, {
            httpPost("/card_tables/lists/${columnId}/subscription.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Unsubscribe from a card column (stop watching for changes)
     * @param columnId The column ID
     */
    suspend fun unsubscribeFromColumn(columnId: Long): Unit {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "UnsubscribeFromCardColumn",
            resourceType = "from_card_column",
            isMutation = true,
            projectId = null,
            resourceId = columnId,
        )
        request(info, {
            httpDelete("/card_tables/lists/${columnId}/subscription.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Create a column in a card table
     * @param cardTableId The card table ID
     * @param body Request body
     */
    suspend fun create(cardTableId: Long, body: CreateCardColumnBody): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "CreateCardColumn",
            resourceType = "card_column",
            isMutation = true,
            projectId = null,
            resourceId = cardTableId,
        )
        return request(info, {
            httpPost("/card_tables/${cardTableId}/columns.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Move a column within a card table
     * @param cardTableId The card table ID
     * @param body Request body
     */
    suspend fun move(cardTableId: Long, body: MoveCardColumnBody): Unit {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "MoveCardColumn",
            resourceType = "card_column",
            isMutation = true,
            projectId = null,
            resourceId = cardTableId,
        )
        request(info, {
            httpPost("/card_tables/${cardTableId}/moves.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("source_id", kotlinx.serialization.json.JsonPrimitive(body.sourceId))
                put("target_id", kotlinx.serialization.json.JsonPrimitive(body.targetId))
                body.position?.let { put("position", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { Unit }
    }
}
