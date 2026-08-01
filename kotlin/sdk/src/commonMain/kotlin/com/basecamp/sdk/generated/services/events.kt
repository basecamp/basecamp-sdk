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
     * @param recordingId The recording ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(recordingId: Long, options: ListEventsOptions? = null): ListResult<Event> {
        val info = OperationInfo(
            service = "Events",
            operation = "ListEvents",
            resourceType = "event",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/recordings/${recordingId}/events.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Event>>(body)
        }
    }

    /**
     * Source-compatibility overload: accepts bare [PaginationOptions].
     *
     * Prefer [ListEventsOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     */
    suspend fun list(recordingId: Long, options: PaginationOptions): ListResult<Event> =
        list(recordingId, ListEventsOptions(maxItems = options.maxItems))
}
