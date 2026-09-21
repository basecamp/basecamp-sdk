package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Subtasks operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class SubtasksService(client: AccountClient) : BaseService(client) {

    /**
     * List a recording's subtasks, in position order
     * @param recordingId The recording ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(recordingId: Long, options: ListSubtasksOptions? = null): ListResult<CardStep> {
        val info = OperationInfo(
            service = "Subtasks",
            operation = "ListSubtasks",
            resourceType = "subtask",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/recordings/${recordingId}/subtasks.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<CardStep>>(body)
        }
    }

    /**
     * Create a subtask under a to-do or a card
     * @param recordingId The recording ID
     * @param body Request body
     */
    suspend fun create(recordingId: Long, body: CreateSubtaskBody): CardStep {
        val info = OperationInfo(
            service = "Subtasks",
            operation = "CreateSubtask",
            resourceType = "subtask",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpPost("/recordings/${recordingId}/subtasks.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
                body.dueOn?.let { put("due_on", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.assigneeIds?.let { put("assignee_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardStep>(body)
        }
    }

    /**
     * Get a subtask by ID
     * @param subtaskId The subtask ID
     */
    suspend fun get(subtaskId: Long): CardStep {
        val info = OperationInfo(
            service = "Subtasks",
            operation = "GetSubtask",
            resourceType = "subtask",
            isMutation = false,
            projectId = null,
            resourceId = subtaskId,
        )
        return request(info, {
            httpGet("/subtasks/${subtaskId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardStep>(body)
        }
    }

    /**
     * Update a subtask
     * @param subtaskId The subtask ID
     * @param body Request body
     */
    suspend fun update(subtaskId: Long, body: UpdateSubtaskBody): CardStep {
        val info = OperationInfo(
            service = "Subtasks",
            operation = "UpdateSubtask",
            resourceType = "subtask",
            isMutation = true,
            projectId = null,
            resourceId = subtaskId,
        )
        return request(info, {
            httpPut("/subtasks/${subtaskId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.dueOn?.let { put("due_on", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.assigneeIds?.let { put("assignee_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardStep>(body)
        }
    }

    /**
     * Delete a subtask
     * @param subtaskId The subtask ID
     */
    suspend fun delete(subtaskId: Long): Unit {
        val info = OperationInfo(
            service = "Subtasks",
            operation = "DeleteSubtask",
            resourceType = "subtask",
            isMutation = true,
            projectId = null,
            resourceId = subtaskId,
        )
        request(info, {
            httpDelete("/subtasks/${subtaskId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Mark a subtask as completed
     * @param subtaskId The subtask ID
     */
    suspend fun complete(subtaskId: Long): Unit {
        val info = OperationInfo(
            service = "Subtasks",
            operation = "CompleteSubtask",
            resourceType = "subtask",
            isMutation = true,
            projectId = null,
            resourceId = subtaskId,
        )
        request(info, {
            httpPost("/subtasks/${subtaskId}/completion.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Mark a subtask as not completed
     * @param subtaskId The subtask ID
     */
    suspend fun uncomplete(subtaskId: Long): Unit {
        val info = OperationInfo(
            service = "Subtasks",
            operation = "UncompleteSubtask",
            resourceType = "subtask",
            isMutation = true,
            projectId = null,
            resourceId = subtaskId,
        )
        request(info, {
            httpDelete("/subtasks/${subtaskId}/completion.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Move a subtask to a new position among its siblings
     * @param subtaskId The subtask ID
     * @param body Request body
     */
    suspend fun reposition(subtaskId: Long, body: RepositionSubtaskBody): Unit {
        val info = OperationInfo(
            service = "Subtasks",
            operation = "RepositionSubtask",
            resourceType = "subtask",
            isMutation = true,
            projectId = null,
            resourceId = subtaskId,
        )
        request(info, {
            httpPut("/subtasks/${subtaskId}/position.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("position", kotlinx.serialization.json.JsonPrimitive(body.position))
            }), operationName = info.operation)
        }) { Unit }
    }
}
