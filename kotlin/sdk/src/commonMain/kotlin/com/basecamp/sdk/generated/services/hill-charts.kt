package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for HillCharts operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class HillChartsService(client: AccountClient) : BaseService(client) {

    /**
     * Get the hill chart for a todoset
     * @param todosetId The todoset ID
     */
    suspend fun get(todosetId: Long): JsonElement {
        val info = OperationInfo(
            service = "HillCharts",
            operation = "GetHillChart",
            resourceType = "hill_chart",
            isMutation = false,
            projectId = null,
            resourceId = todosetId,
        )
        return request(info, {
            httpGet("/todosets/${todosetId}/hill.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Track or untrack todolists on a hill chart
     * @param todosetId The todoset ID
     * @param body Request body
     */
    suspend fun updateSettings(todosetId: Long, body: UpdateHillChartSettingsBody): JsonElement {
        val info = OperationInfo(
            service = "HillCharts",
            operation = "UpdateHillChartSettings",
            resourceType = "hill_chart_setting",
            isMutation = true,
            projectId = null,
            resourceId = todosetId,
        )
        return request(info, {
            httpPut("/todosets/${todosetId}/hills/settings.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.tracked?.let { put("tracked", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.untracked?.let { put("untracked", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
