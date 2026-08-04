package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject

data class PersonProgressResult(
    val events: ListResult<TimelineEvent>,
    val person: Person
)

@Serializable
data class UpcomingScheduleResult(
    @SerialName("schedule_entries") val scheduleEntries: List<UpcomingScheduleEntry>,
    @SerialName("recurring_schedule_entry_occurrences") val recurringScheduleEntryOccurrences: List<UpcomingScheduleEntry>,
    val assignables: List<UpcomingAssignable>
)

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
    suspend fun progress(options: GetProgressReportOptions): ListResult<TimelineEvent> {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetProgressReport",
            resourceType = "progress_report",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/reports/progress.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<TimelineEvent>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [GetProgressReportOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [progress] needs an expected type to disambiguate.
     */
    suspend fun progress(options: PaginationOptions? = null): ListResult<TimelineEvent> =
        progress(GetProgressReportOptions(maxItems = options?.maxItems, page = options?.page))

    /**
     * Get upcoming schedule entries and assignable items within a date window.
     * @param windowStartsOn Inclusive first day of the window, `YYYY-MM-DD`. Required — BC3 answers 400 without it.
     * @param windowEndsOn Inclusive last day of the window, `YYYY-MM-DD`. Required — BC3 answers 400 without it.
     */
    suspend fun upcoming(windowStartsOn: String, windowEndsOn: String): UpcomingScheduleResult {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetUpcomingSchedule",
            resourceType = "upcoming_schedule",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "window_starts_on" to windowStartsOn,
            "window_ends_on" to windowEndsOn,
        )
        return request(info, {
            httpGet("/reports/schedules/upcoming.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<UpcomingScheduleResult>(body)
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
    suspend fun personProgress(personId: Long, options: GetPersonProgressOptions): PersonProgressResult {
        val info = OperationInfo(
            service = "Reports",
            operation = "GetPersonProgress",
            resourceType = "person_progress",
            isMutation = false,
            projectId = null,
            resourceId = personId,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        val (firstPageBody, items) = requestPaginatedWrapped<TimelineEvent>(info, options.toPaginationOptions(), {
            httpGet("/reports/users/progress/${personId}.json" + qs, operationName = info.operation)
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

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [GetPersonProgressOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [personProgress] needs an expected type to disambiguate.
     */
    suspend fun personProgress(personId: Long, options: PaginationOptions? = null): PersonProgressResult =
        personProgress(personId, GetPersonProgressOptions(maxItems = options?.maxItems, page = options?.page))
}
