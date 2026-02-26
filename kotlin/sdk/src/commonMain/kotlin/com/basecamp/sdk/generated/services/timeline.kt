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
    suspend fun projectTimeline(projectId: Long, options: PaginationOptions? = null): ListResult<JsonElement> {
        val info = OperationInfo(
            service = "Timeline",
            operation = "GetProjectTimeline",
            resourceType = "project_timeline",
            isMutation = false,
            projectId = projectId,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/timeline.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<JsonElement>>(body)
        }
    }
}
