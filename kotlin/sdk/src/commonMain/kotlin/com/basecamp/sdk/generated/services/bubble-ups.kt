package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for BubbleUps operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class BubbleUpsService(client: AccountClient) : BaseService(client) {

    /**
     * Bubble up a recording for the current user, resurfacing it in the current
     * @param recordingId The recording ID
     * @param body Request body
     */
    suspend fun createBubbleUp(recordingId: Long, body: CreateBubbleUpBody): Unit {
        val info = OperationInfo(
            service = "BubbleUps",
            operation = "CreateBubbleUp",
            resourceType = "bubble_up",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        request(info, {
            httpPost("/recordings/${recordingId}/bubble_up.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.at?.let { put("at", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * Remove the current user's bubble-up from a recording.
     * @param recordingId The recording ID
     */
    suspend fun deleteBubbleUp(recordingId: Long): Unit {
        val info = OperationInfo(
            service = "BubbleUps",
            operation = "DeleteBubbleUp",
            resourceType = "bubble_up",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        request(info, {
            httpDelete("/recordings/${recordingId}/bubble_up.json", operationName = info.operation)
        }) { Unit }
    }
}
