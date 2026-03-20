package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject

data class PersonProgressResult(
    val events: ListResult<TimelineEvent>,
    val person: Person
)

/**
 * Service for Reports operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class ReportsService(client: AccountClient) : BaseService(client) {

    /**
     * Get the current user's assignments grouped into priorities and non-priorities
     */
    suspend fun myAssignments(): JsonElement {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetMyAssignments",
            resourceType = "my_assignment",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/my/assignments.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get the current user's completed assignments
     */
    suspend fun myAssignmentsCompleted(): List<MyAssignment> {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetMyAssignmentsCompleted",
            resourceType = "my_assignments_completed",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/my/assignments/completed.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<MyAssignment>>(body)
        }
    }

    /**
     * Get the current user's due assignments filtered by scope
     * @param options Optional query parameters and pagination control
     */
    suspend fun myAssignmentsDue(options: GetMyAssignmentsDueOptions? = null): List<MyAssignment> {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetMyAssignmentsDue",
            resourceType = "my_assignments_due",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "scope" to options?.scope,
        )
        return request(info, {
            httpGet("/my/assignments/due.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<MyAssignment>>(body)
        }
    }

    /**
     * Get account-wide activity feed (progress report)
     * @param options Optional query parameters and pagination control
     */
    suspend fun progress(options: PaginationOptions? = null): ListResult<TimelineEvent> {
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
            json.decodeFromString<List<TimelineEvent>>(body)
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
     * @param options Optional query parameters and pagination control
     */
    suspend fun personProgress(personId: Long, options: PaginationOptions? = null): PersonProgressResult {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetPersonProgress",
            resourceType = "person_progress",
            isMutation = false,
            projectId = null,
            resourceId = personId,
        )
        val (firstPageBody, items) = requestPaginatedWrapped<TimelineEvent>(info, options, {
            httpGet("/reports/users/progress/${personId}.json", operationName = info.operation)
        }) { body ->
            json.parseToJsonElement(body).jsonObject["events"]!!
                .jsonArray.map { json.decodeFromJsonElement<TimelineEvent>(it) }
        }
        val wrapper = json.parseToJsonElement(firstPageBody).jsonObject
        return PersonProgressResult(
            events = items,
            person = json.decodeFromJsonElement<Person>(wrapper["person"]!!)
        )
    }
}
