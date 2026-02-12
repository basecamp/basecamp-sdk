package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Schedules operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class SchedulesService(client: AccountClient) : BaseService(client) {

    /**
     * Get a single schedule entry by id.
     * @param projectId The project ID
     * @param entryId The entry ID
     */
    suspend fun getEntry(projectId: Long, entryId: Long): ScheduleEntry {
        val info = OperationInfo(
            service = "Schedules",
            operation = "GetScheduleEntry",
            resourceType = "schedule_entry",
            isMutation = false,
            projectId = projectId,
            resourceId = entryId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/schedule_entries/${entryId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<ScheduleEntry>(body)
        }
    }

    /**
     * Update an existing schedule entry
     * @param projectId The project ID
     * @param entryId The entry ID
     * @param body Request body
     */
    suspend fun updateEntry(projectId: Long, entryId: Long, body: UpdateScheduleEntryBody): ScheduleEntry {
        val info = OperationInfo(
            service = "Schedules",
            operation = "UpdateScheduleEntry",
            resourceType = "schedule_entry",
            isMutation = true,
            projectId = projectId,
            resourceId = entryId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/schedule_entries/${entryId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.summary?.let { put("summary", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.startsAt?.let { put("starts_at", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.endsAt?.let { put("ends_at", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.participantIds?.let { put("participant_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.allDay?.let { put("all_day", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.notify?.let { put("notify", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<ScheduleEntry>(body)
        }
    }

    /**
     * Get a specific occurrence of a recurring schedule entry
     * @param projectId The project ID
     * @param entryId The entry ID
     * @param date The date
     */
    suspend fun getEntryOccurrence(projectId: Long, entryId: Long, date: String): ScheduleEntry {
        val info = OperationInfo(
            service = "Schedules",
            operation = "GetScheduleEntryOccurrence",
            resourceType = "schedule_entry_occurrence",
            isMutation = false,
            projectId = projectId,
            resourceId = entryId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/schedule_entries/${entryId}/occurrences/${date}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<ScheduleEntry>(body)
        }
    }

    /**
     * Get a schedule
     * @param projectId The project ID
     * @param scheduleId The schedule ID
     */
    suspend fun get(projectId: Long, scheduleId: Long): Schedule {
        val info = OperationInfo(
            service = "Schedules",
            operation = "GetSchedule",
            resourceType = "schedule",
            isMutation = false,
            projectId = projectId,
            resourceId = scheduleId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/schedules/${scheduleId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Schedule>(body)
        }
    }

    /**
     * Update schedule settings
     * @param projectId The project ID
     * @param scheduleId The schedule ID
     * @param body Request body
     */
    suspend fun updateSettings(projectId: Long, scheduleId: Long, body: UpdateScheduleSettingsBody): Schedule {
        val info = OperationInfo(
            service = "Schedules",
            operation = "UpdateScheduleSettings",
            resourceType = "schedule_setting",
            isMutation = true,
            projectId = projectId,
            resourceId = scheduleId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/schedules/${scheduleId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("include_due_assignments", kotlinx.serialization.json.JsonPrimitive(body.includeDueAssignments))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Schedule>(body)
        }
    }

    /**
     * List entries on a schedule
     * @param projectId The project ID
     * @param scheduleId The schedule ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listEntries(projectId: Long, scheduleId: Long, options: ListScheduleEntriesOptions? = null): ListResult<ScheduleEntry> {
        val info = OperationInfo(
            service = "Schedules",
            operation = "ListScheduleEntries",
            resourceType = "schedule_entry",
            isMutation = false,
            projectId = projectId,
            resourceId = scheduleId,
        )
        val qs = buildQueryString(
            "status" to options?.status,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/buckets/${projectId}/schedules/${scheduleId}/entries.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<ScheduleEntry>>(body)
        }
    }

    /**
     * Create a new schedule entry
     * @param projectId The project ID
     * @param scheduleId The schedule ID
     * @param body Request body
     */
    suspend fun createEntry(projectId: Long, scheduleId: Long, body: CreateScheduleEntryBody): ScheduleEntry {
        val info = OperationInfo(
            service = "Schedules",
            operation = "CreateScheduleEntry",
            resourceType = "schedule_entry",
            isMutation = true,
            projectId = projectId,
            resourceId = scheduleId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/schedules/${scheduleId}/entries.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("summary", kotlinx.serialization.json.JsonPrimitive(body.summary))
                put("starts_at", kotlinx.serialization.json.JsonPrimitive(body.startsAt))
                put("ends_at", kotlinx.serialization.json.JsonPrimitive(body.endsAt))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.participantIds?.let { put("participant_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.allDay?.let { put("all_day", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.notify?.let { put("notify", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<ScheduleEntry>(body)
        }
    }
}
