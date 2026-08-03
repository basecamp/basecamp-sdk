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
    suspend fun projectTimeline(projectId: Long, options: GetProjectTimelineOptions): ListResult<TimelineEvent> {
        val info = OperationInfo(
            service = "Timeline",
            operation = "GetProjectTimeline",
            resourceType = "project_timeline",
            isMutation = false,
            projectId = projectId,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/projects/${projectId}/timeline.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<TimelineEvent>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [GetProjectTimelineOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [projectTimeline] needs an expected type to disambiguate.
     */
    suspend fun projectTimeline(projectId: Long, options: PaginationOptions? = null): ListResult<TimelineEvent> =
        projectTimeline(projectId, GetProjectTimelineOptions(maxItems = options?.maxItems, page = options?.page))
}
