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
    suspend fun get(todosetId: Long): HillChart {
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
            json.decodeFromString<HillChart>(body)
        }
    }

    /**
     * Track or untrack todolists on a hill chart
     * @param todosetId The todoset ID
     * @param body Request body
     */
    suspend fun updateSettings(todosetId: Long, body: UpdateHillChartSettingsBody): HillChart {
        val info = OperationInfo(
            service = "HillCharts",
            operation = "UpdateHillChartSettings",
            resourceType = "hill_chart",
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
            json.decodeFromString<HillChart>(body)
        }
    }

    /**
     * List the progress updates on a todoset's hill chart, newest first.
     * @param todosetId The todoset ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listVersions(todosetId: Long, options: ListHillChartVersionsOptions? = null): ListResult<HillChartVersion> {
        val info = OperationInfo(
            service = "HillCharts",
            operation = "ListHillChartVersions",
            resourceType = "hill_chart_version",
            isMutation = false,
            projectId = null,
            resourceId = todosetId,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/todosets/${todosetId}/hills/versions.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<HillChartVersion>>(body)
        }
    }

    /**
     * Post a progress update on a todoset's hill chart.
     * @param todosetId The todoset ID
     * @param body Request body
     */
    suspend fun createVersion(todosetId: Long, body: CreateHillChartVersionBody): HillChartVersion {
        val info = OperationInfo(
            service = "HillCharts",
            operation = "CreateHillChartVersion",
            resourceType = "hill_chart_version",
            isMutation = true,
            projectId = null,
            resourceId = todosetId,
        )
        return request(info, {
            httpPost("/todosets/${todosetId}/hills/versions.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.dots?.let { put("dots", kotlinx.serialization.json.JsonArray(it)) }
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<HillChartVersion>(body)
        }
    }

    /**
     * Get one progress update from a todoset's hill chart
     * @param todosetId The todoset ID
     * @param versionId The version ID
     */
    suspend fun getVersion(todosetId: Long, versionId: Long): HillChartVersion {
        val info = OperationInfo(
            service = "HillCharts",
            operation = "GetHillChartVersion",
            resourceType = "hill_chart_version",
            isMutation = false,
            projectId = null,
            resourceId = versionId,
        )
        return request(info, {
            httpGet("/todosets/${todosetId}/hills/versions/${versionId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<HillChartVersion>(body)
        }
    }
}
