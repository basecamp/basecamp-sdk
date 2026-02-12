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
     * @param projectId The project ID
     * @param columnId The column ID
     */
    suspend fun get(projectId: Long, columnId: Long): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "GetCardColumn",
            resourceType = "card_column",
            isMutation = false,
            projectId = projectId,
            resourceId = columnId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/card_tables/columns/${columnId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Update an existing column
     * @param projectId The project ID
     * @param columnId The column ID
     * @param body Request body
     */
    suspend fun update(projectId: Long, columnId: Long, body: UpdateCardColumnBody): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "UpdateCardColumn",
            resourceType = "card_column",
            isMutation = true,
            projectId = projectId,
            resourceId = columnId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/card_tables/columns/${columnId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Set the color of a column
     * @param projectId The project ID
     * @param columnId The column ID
     * @param body Request body
     */
    suspend fun setColor(projectId: Long, columnId: Long, body: SetCardColumnColorBody): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "SetCardColumnColor",
            resourceType = "card_column_color",
            isMutation = true,
            projectId = projectId,
            resourceId = columnId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/card_tables/columns/${columnId}/color.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("color", kotlinx.serialization.json.JsonPrimitive(body.color))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Enable on-hold section in a column
     * @param projectId The project ID
     * @param columnId The column ID
     */
    suspend fun enableOnHold(projectId: Long, columnId: Long): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "EnableCardColumnOnHold",
            resourceType = "card_column_on_hold",
            isMutation = true,
            projectId = projectId,
            resourceId = columnId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/card_tables/columns/${columnId}/on_hold.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Disable on-hold section in a column
     * @param projectId The project ID
     * @param columnId The column ID
     */
    suspend fun disableOnHold(projectId: Long, columnId: Long): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "DisableCardColumnOnHold",
            resourceType = "card_column_on_hold",
            isMutation = true,
            projectId = projectId,
            resourceId = columnId,
        )
        return request(info, {
            httpDelete("/buckets/${projectId}/card_tables/columns/${columnId}/on_hold.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Subscribe to a card column (watch for changes)
     * @param projectId The project ID
     * @param columnId The column ID
     */
    suspend fun subscribeToColumn(projectId: Long, columnId: Long): Unit {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "SubscribeToCardColumn",
            resourceType = "to_card_column",
            isMutation = true,
            projectId = projectId,
            resourceId = columnId,
        )
        request(info, {
            httpPost("/buckets/${projectId}/card_tables/lists/${columnId}/subscription.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Unsubscribe from a card column (stop watching for changes)
     * @param projectId The project ID
     * @param columnId The column ID
     */
    suspend fun unsubscribeFromColumn(projectId: Long, columnId: Long): Unit {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "UnsubscribeFromCardColumn",
            resourceType = "from_card_column",
            isMutation = true,
            projectId = projectId,
            resourceId = columnId,
        )
        request(info, {
            httpDelete("/buckets/${projectId}/card_tables/lists/${columnId}/subscription.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Create a column in a card table
     * @param projectId The project ID
     * @param cardTableId The card table ID
     * @param body Request body
     */
    suspend fun create(projectId: Long, cardTableId: Long, body: CreateCardColumnBody): CardColumn {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "CreateCardColumn",
            resourceType = "card_column",
            isMutation = true,
            projectId = projectId,
            resourceId = cardTableId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/card_tables/${cardTableId}/columns.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardColumn>(body)
        }
    }

    /**
     * Move a column within a card table
     * @param projectId The project ID
     * @param cardTableId The card table ID
     * @param body Request body
     */
    suspend fun move(projectId: Long, cardTableId: Long, body: MoveCardColumnBody): Unit {
        val info = OperationInfo(
            service = "CardColumns",
            operation = "MoveCardColumn",
            resourceType = "card_column",
            isMutation = true,
            projectId = projectId,
            resourceId = cardTableId,
        )
        request(info, {
            httpPost("/buckets/${projectId}/card_tables/${cardTableId}/moves.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("source_id", kotlinx.serialization.json.JsonPrimitive(body.sourceId))
                put("target_id", kotlinx.serialization.json.JsonPrimitive(body.targetId))
                body.position?.let { put("position", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { Unit }
    }
}
