package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Lineup operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class LineupService(client: AccountClient) : BaseService(client) {

    /**
     * Create a new lineup marker
     * @param body Request body
     */
    suspend fun create(body: CreateLineupMarkerBody): Unit {
        val info = OperationInfo(
            service = "Lineup",
            operation = "CreateLineupMarker",
            resourceType = "lineup_marker",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        request(info, {
            httpPost("/lineup/markers.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
                put("date", kotlinx.serialization.json.JsonPrimitive(body.date))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * Update an existing lineup marker
     * @param markerId The marker ID
     * @param body Request body
     */
    suspend fun update(markerId: Long, body: UpdateLineupMarkerBody): Unit {
        val info = OperationInfo(
            service = "Lineup",
            operation = "UpdateLineupMarker",
            resourceType = "lineup_marker",
            isMutation = true,
            projectId = null,
            resourceId = markerId,
        )
        request(info, {
            httpPut("/lineup/markers/${markerId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.name?.let { put("name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.date?.let { put("date", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * Delete a lineup marker
     * @param markerId The marker ID
     */
    suspend fun delete(markerId: Long): Unit {
        val info = OperationInfo(
            service = "Lineup",
            operation = "DeleteLineupMarker",
            resourceType = "lineup_marker",
            isMutation = true,
            projectId = null,
            resourceId = markerId,
        )
        request(info, {
            httpDelete("/lineup/markers/${markerId}", operationName = info.operation)
        }) { Unit }
    }
}
