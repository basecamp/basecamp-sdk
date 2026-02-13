package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Reports operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class ReportsService(client: AccountClient) : BaseService(client) {

    /**
     * Get account-wide activity feed (progress report)
     * @param options Optional query parameters and pagination control
     */
    suspend fun progress(options: PaginationOptions? = null): ListResult<JsonElement> {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetProgressReport",
            resourceType = "progress_report",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/reports/progress.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<JsonElement>>(body)
        }
    }

    /**
     * Get upcoming schedule entries within a date window
     * @param options Optional query parameters and pagination control
     */
    suspend fun upcoming(options: GetUpcomingScheduleOptions? = null): JsonElement {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetUpcomingSchedule",
            resourceType = "upcoming_schedule",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "window_starts_on" to options?.windowStartsOn,
            "window_ends_on" to options?.windowEndsOn,
        )
        return request(info, {
            httpGet("/reports/schedules/upcoming.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get todos assigned to a specific person
     * @param personId The person ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun assigned(personId: Long, options: GetAssignedTodosOptions? = null): JsonElement {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetAssignedTodos",
            resourceType = "assigned_todo",
            isMutation = false,
            projectId = null,
            resourceId = personId,
        )
        val qs = buildQueryString(
            "group_by" to options?.groupBy,
        )
        return request(info, {
            httpGet("/reports/todos/assigned/${personId}" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get overdue todos grouped by lateness
     */
    suspend fun overdue(): JsonElement {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetOverdueTodos",
            resourceType = "overdue_todo",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/reports/todos/overdue.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get a person's activity timeline
     * @param personId The person ID
     */
    suspend fun personProgress(personId: Long): JsonElement {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetPersonProgress",
            resourceType = "person_progress",
            isMutation = false,
            projectId = null,
            resourceId = personId,
        )
        return request(info, {
            httpGet("/reports/users/progress/${personId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
