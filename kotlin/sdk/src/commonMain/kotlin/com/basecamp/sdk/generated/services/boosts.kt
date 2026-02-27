package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Boosts operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class BoostsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a single boost
     * @param boostId The boost ID
     */
    suspend fun get(boostId: Long): Boost {
        val info = OperationInfo(
            service = "Boosts",
            operation = "GetBoost",
            resourceType = "boost",
            isMutation = false,
            projectId = null,
            resourceId = boostId,
        )
        return request(info, {
            httpGet("/boosts/${boostId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Boost>(body)
        }
    }

    /**
     * Delete a boost
     * @param boostId The boost ID
     */
    suspend fun delete(boostId: Long): Unit {
        val info = OperationInfo(
            service = "Boosts",
            operation = "DeleteBoost",
            resourceType = "boost",
            isMutation = true,
            projectId = null,
            resourceId = boostId,
        )
        request(info, {
            httpDelete("/boosts/${boostId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * List boosts on a recording
     * @param recordingId The recording ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listForRecording(recordingId: Long, options: PaginationOptions? = null): ListResult<Boost> {
        val info = OperationInfo(
            service = "Boosts",
            operation = "ListRecordingBoosts",
            resourceType = "recording_boost",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        return requestPaginated(info, options, {
            httpGet("/recordings/${recordingId}/boosts.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Boost>>(body)
        }
    }

    /**
     * Create a boost on a recording
     * @param recordingId The recording ID
     * @param body Request body
     */
    suspend fun createForRecording(recordingId: Long, body: CreateRecordingBoostBody): Boost {
        val info = OperationInfo(
            service = "Boosts",
            operation = "CreateRecordingBoost",
            resourceType = "recording_boost",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpPost("/recordings/${recordingId}/boosts.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Boost>(body)
        }
    }

    /**
     * List boosts on a specific event within a recording
     * @param recordingId The recording ID
     * @param eventId The event ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listForEvent(recordingId: Long, eventId: Long, options: PaginationOptions? = null): ListResult<Boost> {
        val info = OperationInfo(
            service = "Boosts",
            operation = "ListEventBoosts",
            resourceType = "event_boost",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        return requestPaginated(info, options, {
            httpGet("/recordings/${recordingId}/events/${eventId}/boosts.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Boost>>(body)
        }
    }

    /**
     * Create a boost on a specific event within a recording
     * @param recordingId The recording ID
     * @param eventId The event ID
     * @param body Request body
     */
    suspend fun createForEvent(recordingId: Long, eventId: Long, body: CreateEventBoostBody): Boost {
        val info = OperationInfo(
            service = "Boosts",
            operation = "CreateEventBoost",
            resourceType = "event_boost",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpPost("/recordings/${recordingId}/events/${eventId}/boosts.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Boost>(body)
        }
    }
}
