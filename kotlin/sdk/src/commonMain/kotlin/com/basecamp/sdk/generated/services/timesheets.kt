package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Timesheets operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class TimesheetsService(client: AccountClient) : BaseService(client) {

    /**
     * Get timesheet for a specific recording
     * @param recordingId The recording ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun forRecording(recordingId: Long, options: GetRecordingTimesheetOptions? = null): JsonElement {
        val info = OperationInfo(
            service = "Timesheets",
            operation = "GetRecordingTimesheet",
            resourceType = "recording_timesheet",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        val qs = buildQueryString(
            "from" to options?.from,
            "to" to options?.to,
            "person_id" to options?.personId,
        )
        return request(info, {
            httpGet("/recordings/${recordingId}/timesheet.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Create a timesheet entry on a recording
     * @param recordingId The recording ID
     * @param body Request body
     */
    suspend fun create(recordingId: Long, body: CreateTimesheetEntryBody): JsonElement {
        val info = OperationInfo(
            service = "Timesheets",
            operation = "CreateTimesheetEntry",
            resourceType = "timesheet_entry",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpPost("/recordings/${recordingId}/timesheet/entries.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("date", kotlinx.serialization.json.JsonPrimitive(body.date))
                put("hours", kotlinx.serialization.json.JsonPrimitive(body.hours))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.personId?.let { put("person_id", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get account-wide timesheet report
     * @param options Optional query parameters and pagination control
     */
    suspend fun report(options: GetTimesheetReportOptions? = null): JsonElement {
        val info = OperationInfo(
            service = "Timesheets",
            operation = "GetTimesheetReport",
            resourceType = "timesheet_report",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "from" to options?.from,
            "to" to options?.to,
            "person_id" to options?.personId,
        )
        return request(info, {
            httpGet("/reports/timesheet.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get timesheet for a specific project
     * @param options Optional query parameters and pagination control
     */
    suspend fun forProject(options: GetProjectTimesheetOptions? = null): JsonElement {
        val info = OperationInfo(
            service = "Timesheets",
            operation = "GetProjectTimesheet",
            resourceType = "project_timesheet",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "from" to options?.from,
            "to" to options?.to,
            "person_id" to options?.personId,
        )
        return request(info, {
            httpGet("/timesheet.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get a single timesheet entry
     * @param entryId The entry ID
     */
    suspend fun get(entryId: Long): JsonElement {
        val info = OperationInfo(
            service = "Timesheets",
            operation = "GetTimesheetEntry",
            resourceType = "timesheet_entry",
            isMutation = false,
            projectId = null,
            resourceId = entryId,
        )
        return request(info, {
            httpGet("/timesheet/entries/${entryId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Update a timesheet entry
     * @param entryId The entry ID
     * @param body Request body
     */
    suspend fun update(entryId: Long, body: UpdateTimesheetEntryBody): JsonElement {
        val info = OperationInfo(
            service = "Timesheets",
            operation = "UpdateTimesheetEntry",
            resourceType = "timesheet_entry",
            isMutation = true,
            projectId = null,
            resourceId = entryId,
        )
        return request(info, {
            httpPut("/timesheet/entries/${entryId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.date?.let { put("date", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.hours?.let { put("hours", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.personId?.let { put("person_id", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
