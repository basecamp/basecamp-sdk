package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for MyAssignments operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class MyAssignmentsService(client: AccountClient) : BaseService(client) {

    /**
     * Get the current user's active assignments grouped into priorities and non_priorities.
     */
    suspend fun myAssignments(): JsonElement {
        val info = OperationInfo(
            service = "MyAssignments",
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
     * Get the current user's completed assignments.
     */
    suspend fun myCompletedAssignments(): JsonElement {
        val info = OperationInfo(
            service = "MyAssignments",
            operation = "GetMyCompletedAssignments",
            resourceType = "my_completed_assignment",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/my/assignments/completed.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get the current user's assignments filtered by due date scope.
     * @param options Optional query parameters and pagination control
     */
    suspend fun myDueAssignments(options: GetMyDueAssignmentsOptions? = null): JsonElement {
        val info = OperationInfo(
            service = "MyAssignments",
            operation = "GetMyDueAssignments",
            resourceType = "my_due_assignment",
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
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Add a recording to Up Next — the current user's ordered list of prioritized
     * @param body Request body
     */
    suspend fun prioritizeAssignment(body: PrioritizeAssignmentBody): Unit {
        val info = OperationInfo(
            service = "MyAssignments",
            operation = "PrioritizeAssignment",
            resourceType = "resource",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        request(info, {
            httpPost("/my/priorities.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("id", kotlinx.serialization.json.JsonPrimitive(body.id))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * Remove a recording from Up Next. Exact-target:
     * @param recordingId The recording ID
     */
    suspend fun deprioritizeAssignment(recordingId: Long): Unit {
        val info = OperationInfo(
            service = "MyAssignments",
            operation = "DeprioritizeAssignment",
            resourceType = "resource",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        request(info, {
            httpDelete("/my/priorities/${recordingId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Move an already-prioritized recording to a new 1-based position in Up Next
     * @param body Request body
     */
    suspend fun reorderUpNext(body: ReorderUpNextBody): Unit {
        val info = OperationInfo(
            service = "MyAssignments",
            operation = "ReorderUpNext",
            resourceType = "resource",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        request(info, {
            httpPost("/my/priority_moves.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("source_id", kotlinx.serialization.json.JsonPrimitive(body.sourceId))
                put("position", kotlinx.serialization.json.JsonPrimitive(body.position))
            }), operationName = info.operation)
        }) { Unit }
    }
}
