package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Tools operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class ToolsService(client: AccountClient) : BaseService(client) {

    /**
     * Clone an existing tool to create a new one
     * @param body Request body
     */
    suspend fun clone(body: CloneToolBody): Tool {
        val info = OperationInfo(
            service = "Tools",
            operation = "CloneTool",
            resourceType = "tool",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/dock/tools.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("source_recording_id", kotlinx.serialization.json.JsonPrimitive(body.sourceRecordingId))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Tool>(body)
        }
    }

    /**
     * Get a dock tool by id
     * @param toolId The tool ID
     */
    suspend fun get(toolId: Long): Tool {
        val info = OperationInfo(
            service = "Tools",
            operation = "GetTool",
            resourceType = "tool",
            isMutation = false,
            projectId = null,
            resourceId = toolId,
        )
        return request(info, {
            httpGet("/dock/tools/${toolId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Tool>(body)
        }
    }

    /**
     * Update (rename) an existing tool
     * @param toolId The tool ID
     * @param body Request body
     */
    suspend fun update(toolId: Long, body: UpdateToolBody): Tool {
        val info = OperationInfo(
            service = "Tools",
            operation = "UpdateTool",
            resourceType = "tool",
            isMutation = true,
            projectId = null,
            resourceId = toolId,
        )
        return request(info, {
            httpPut("/dock/tools/${toolId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Tool>(body)
        }
    }

    /**
     * Delete a tool (trash it)
     * @param toolId The tool ID
     */
    suspend fun delete(toolId: Long): Unit {
        val info = OperationInfo(
            service = "Tools",
            operation = "DeleteTool",
            resourceType = "tool",
            isMutation = true,
            projectId = null,
            resourceId = toolId,
        )
        request(info, {
            httpDelete("/dock/tools/${toolId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Enable a tool (show it on the project dock)
     * @param toolId The tool ID
     */
    suspend fun enable(toolId: Long): Unit {
        val info = OperationInfo(
            service = "Tools",
            operation = "EnableTool",
            resourceType = "tool",
            isMutation = true,
            projectId = null,
            resourceId = toolId,
        )
        request(info, {
            httpPost("/recordings/${toolId}/position.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Reposition a tool on the project dock
     * @param toolId The tool ID
     * @param body Request body
     */
    suspend fun reposition(toolId: Long, body: RepositionToolBody): Unit {
        val info = OperationInfo(
            service = "Tools",
            operation = "RepositionTool",
            resourceType = "tool",
            isMutation = true,
            projectId = null,
            resourceId = toolId,
        )
        request(info, {
            httpPut("/recordings/${toolId}/position.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("position", kotlinx.serialization.json.JsonPrimitive(body.position))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * Disable a tool (hide it from the project dock)
     * @param toolId The tool ID
     */
    suspend fun disable(toolId: Long): Unit {
        val info = OperationInfo(
            service = "Tools",
            operation = "DisableTool",
            resourceType = "tool",
            isMutation = true,
            projectId = null,
            resourceId = toolId,
        )
        request(info, {
            httpDelete("/recordings/${toolId}/position.json", operationName = info.operation)
        }) { Unit }
    }
}
