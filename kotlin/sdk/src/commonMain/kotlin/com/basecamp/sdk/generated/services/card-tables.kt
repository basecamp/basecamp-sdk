package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for CardTables operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class CardTablesService(client: AccountClient) : BaseService(client) {

    /**
     * Get a card table by ID
     * @param cardTableId The card table ID
     */
    suspend fun get(cardTableId: Long): CardTable {
        val info = OperationInfo(
            service = "CardTables",
            operation = "GetCardTable",
            resourceType = "card_table",
            isMutation = false,
            projectId = null,
            resourceId = cardTableId,
        )
        return request(info, {
            httpGet("/card_tables/${cardTableId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardTable>(body)
        }
    }
}
