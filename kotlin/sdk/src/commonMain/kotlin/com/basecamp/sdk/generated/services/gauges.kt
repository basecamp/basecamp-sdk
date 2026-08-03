package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Gauges operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class GaugesService(client: AccountClient) : BaseService(client) {

    /**
     * Get a gauge needle by ID
     * @param needleId The needle ID
     */
    suspend fun gaugeNeedle(needleId: Long): JsonElement {
        val info = OperationInfo(
            service = "Gauges",
            operation = "GetGaugeNeedle",
            resourceType = "gauge_needle",
            isMutation = false,
            projectId = null,
            resourceId = needleId,
        )
        return request(info, {
            httpGet("/gauge_needles/${needleId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Update a gauge needle's description. Position and color are immutable.
     * @param needleId The needle ID
     * @param body Request body
     */
    suspend fun updateGaugeNeedle(needleId: Long, body: UpdateGaugeNeedleBody): JsonElement {
        val info = OperationInfo(
            service = "Gauges",
            operation = "UpdateGaugeNeedle",
            resourceType = "gauge_needle",
            isMutation = true,
            projectId = null,
            resourceId = needleId,
        )
        return request(info, {
            httpPut("/gauge_needles/${needleId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.gaugeNeedle?.let { put("gauge_needle", it) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Destroy a gauge needle
     * @param needleId The needle ID
     */
    suspend fun destroyGaugeNeedle(needleId: Long): Unit {
        val info = OperationInfo(
            service = "Gauges",
            operation = "DestroyGaugeNeedle",
            resourceType = "resource",
            isMutation = true,
            projectId = null,
            resourceId = needleId,
        )
        request(info, {
            httpDelete("/gauge_needles/${needleId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Enable or disable the gauge for a project. Only project admins can toggle gauges.
     * @param projectId The project ID
     * @param body Request body
     */
    suspend fun toggleGauge(projectId: Long, body: ToggleGaugeBody): Unit {
        val info = OperationInfo(
            service = "Gauges",
            operation = "ToggleGauge",
            resourceType = "resource",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        request(info, {
            httpPut("/projects/${projectId}/gauge.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("gauge", body.gauge)
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * List gauge needles for a project, ordered newest first.
     * @param projectId The project ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listGaugeNeedles(projectId: Long, options: ListGaugeNeedlesOptions): ListResult<JsonElement> {
        val info = OperationInfo(
            service = "Gauges",
            operation = "ListGaugeNeedles",
            resourceType = "gauge_needle",
            isMutation = false,
            projectId = projectId,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/projects/${projectId}/gauge/needles.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<JsonElement>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [ListGaugeNeedlesOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [listGaugeNeedles] needs an expected type to disambiguate.
     */
    suspend fun listGaugeNeedles(projectId: Long, options: PaginationOptions? = null): ListResult<JsonElement> =
        listGaugeNeedles(projectId, ListGaugeNeedlesOptions(maxItems = options?.maxItems, page = options?.page))

    /**
     * Create a gauge needle (progress update) for a project
     * @param projectId The project ID
     * @param body Request body
     */
    suspend fun createGaugeNeedle(projectId: Long, body: CreateGaugeNeedleBody): JsonElement {
        val info = OperationInfo(
            service = "Gauges",
            operation = "CreateGaugeNeedle",
            resourceType = "gauge_needle",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        return request(info, {
            httpPost("/projects/${projectId}/gauge/needles.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("gauge_needle", body.gaugeNeedle)
                body.notify?.let { put("notify", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * List gauges across all projects the authenticated user has access to.
     * @param options Optional query parameters and pagination control
     */
    suspend fun listGauges(options: ListGaugesOptions? = null): ListResult<JsonElement> {
        val info = OperationInfo(
            service = "Gauges",
            operation = "ListGauges",
            resourceType = "gauge",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "bucket_ids" to options?.bucketIds,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/reports/gauges.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<JsonElement>>(body)
        }
    }
}
