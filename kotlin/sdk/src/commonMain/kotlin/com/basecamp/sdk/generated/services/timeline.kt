package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Timeline operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class TimelineService(client: AccountClient) : BaseService(client) {

    /**
     * Get project timeline
     * @param projectId The project ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun projectTimeline(projectId: Long, options: GetProjectTimelineOptions? = null): ListResult<TimelineEvent> {
        val info = OperationInfo(
            service = "Timeline",
            operation = "GetProjectTimeline",
            resourceType = "project_timeline",
            isMutation = false,
            projectId = projectId,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/projects/${projectId}/timeline.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<TimelineEvent>>(body)
        }
    }
}
