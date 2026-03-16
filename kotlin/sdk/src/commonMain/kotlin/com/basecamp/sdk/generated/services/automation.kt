package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Automation operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class AutomationService(client: AccountClient) : BaseService(client) {

    /**
     * List all lineup markers for the account
     */
    suspend fun listLineupMarkers(): List<LineupMarker> {
        val info = OperationInfo(
            service = "Automation",
            operation = "ListLineupMarkers",
            resourceType = "lineup_marker",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/lineup/markers.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<LineupMarker>>(body)
        }
    }
}
