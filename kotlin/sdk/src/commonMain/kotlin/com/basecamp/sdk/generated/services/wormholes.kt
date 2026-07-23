package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Wormholes operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class WormholesService(client: AccountClient) : BaseService(client) {

    /**
     * Update a wormhole's destination column
     * @param bucketId The bucket ID
     * @param wormholeId The wormhole ID
     * @param body Request body
     */
    suspend fun update(bucketId: Long, wormholeId: Long, body: UpdateWormholeBody): Wormhole {
        val info = OperationInfo(
            service = "Wormholes",
            operation = "UpdateWormhole",
            resourceType = "wormhole",
            isMutation = true,
            projectId = null,
            resourceId = wormholeId,
        )
        return request(info, {
            httpPut("/buckets/${bucketId}/card_tables/wormholes/${wormholeId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("destination_recording_id", kotlinx.serialization.json.JsonPrimitive(body.destinationRecordingId))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Wormhole>(body)
        }
    }

    /**
     * Delete a wormhole
     * @param bucketId The bucket ID
     * @param wormholeId The wormhole ID
     */
    suspend fun delete(bucketId: Long, wormholeId: Long): Unit {
        val info = OperationInfo(
            service = "Wormholes",
            operation = "DeleteWormhole",
            resourceType = "wormhole",
            isMutation = true,
            projectId = null,
            resourceId = wormholeId,
        )
        request(info, {
            httpDelete("/buckets/${bucketId}/card_tables/wormholes/${wormholeId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Create a wormhole linking this card table to a column on another card table.
     * @param bucketId The bucket ID
     * @param cardTableId The card table ID
     * @param body Request body
     */
    suspend fun create(bucketId: Long, cardTableId: Long, body: CreateWormholeBody): Wormhole {
        val info = OperationInfo(
            service = "Wormholes",
            operation = "CreateWormhole",
            resourceType = "wormhole",
            isMutation = true,
            projectId = null,
            resourceId = cardTableId,
        )
        return request(info, {
            httpPost("/buckets/${bucketId}/card_tables/${cardTableId}/wormholes.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("destination_recording_id", kotlinx.serialization.json.JsonPrimitive(body.destinationRecordingId))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Wormhole>(body)
        }
    }
}
