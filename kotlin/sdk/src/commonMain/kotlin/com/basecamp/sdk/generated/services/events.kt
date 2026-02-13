package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Events operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class EventsService(client: AccountClient) : BaseService(client) {

    /**
     * List all events for a recording
     * @param projectId The project ID
     * @param recordingId The recording ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(projectId: Long, recordingId: Long, options: PaginationOptions? = null): ListResult<Event> {
        val info = OperationInfo(
            service = "Events",
            operation = "ListEvents",
            resourceType = "event",
            isMutation = false,
            projectId = projectId,
            resourceId = recordingId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/recordings/${recordingId}/events.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Event>>(body)
        }
    }
}
