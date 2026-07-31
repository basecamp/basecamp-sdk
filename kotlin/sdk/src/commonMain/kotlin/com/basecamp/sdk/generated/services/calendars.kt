package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Calendars operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class CalendarsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a calendar by its bucket id. A Calendar is a top-level BC5 bucketable
     * @param calendarId The calendar ID
     */
    suspend fun getCalendar(calendarId: Long): Calendar {
        val info = OperationInfo(
            service = "Calendars",
            operation = "GetCalendar",
            resourceType = "calendar",
            isMutation = false,
            projectId = null,
            resourceId = calendarId,
        )
        return request(info, {
            httpGet("/calendars/${calendarId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Calendar>(body)
        }
    }

    /**
     * Update a calendar's display color. An unknown color returns 422 with a JSON
     * @param calendarId The calendar ID
     * @param body Request body
     */
    suspend fun updateCalendar(calendarId: Long, body: UpdateCalendarBody): Calendar {
        val info = OperationInfo(
            service = "Calendars",
            operation = "UpdateCalendar",
            resourceType = "calendar",
            isMutation = true,
            projectId = null,
            resourceId = calendarId,
        )
        return request(info, {
            httpPut("/calendars/${calendarId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("calendar", body.calendar)
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Calendar>(body)
        }
    }
}
