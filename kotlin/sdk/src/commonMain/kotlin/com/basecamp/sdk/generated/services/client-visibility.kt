package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for ClientVisibility operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class ClientVisibilityService(client: AccountClient) : BaseService(client) {

    /**
     * Set client visibility for a recording
     * @param projectId The project ID
     * @param recordingId The recording ID
     * @param body Request body
     */
    suspend fun setVisibility(projectId: Long, recordingId: Long, body: SetClientVisibilityBody): Recording {
        val info = OperationInfo(
            service = "ClientVisibility",
            operation = "SetClientVisibility",
            resourceType = "client_visibility",
            isMutation = true,
            projectId = projectId,
            resourceId = recordingId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/recordings/${recordingId}/client_visibility.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("visible_to_clients", kotlinx.serialization.json.JsonPrimitive(body.visibleToClients))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Recording>(body)
        }
    }
}
