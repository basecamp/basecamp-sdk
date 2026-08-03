package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Folders operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class FoldersService(client: AccountClient) : BaseService(client) {

    /**
     * List the authenticated user's folders in home-screen order.
     */
    suspend fun listFolders(): List<Folder> {
        val info = OperationInfo(
            service = "Folders",
            operation = "ListFolders",
            resourceType = "folder",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/stacks.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Folder>>(body)
        }
    }

    /**
     * Create a folder for the authenticated user and file the given projects into it.
     * @param body Request body
     */
    suspend fun createFolder(body: CreateFolderBody): FolderWithProjects {
        val info = OperationInfo(
            service = "Folders",
            operation = "CreateFolder",
            resourceType = "folder",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/stacks.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.name?.let { put("name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.projectIds?.let { put("project_ids", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<FolderWithProjects>(body)
        }
    }

    /**
     * Get one folder, with the projects grouped inside it expanded under `projects`.
     * @param folderId The folder ID
     */
    suspend fun getFolder(folderId: Long): FolderWithProjects {
        val info = OperationInfo(
            service = "Folders",
            operation = "GetFolder",
            resourceType = "folder",
            isMutation = false,
            projectId = null,
            resourceId = folderId,
        )
        return request(info, {
            httpGet("/stacks/${folderId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<FolderWithProjects>(body)
        }
    }

    /**
     * Rename a folder.
     * @param folderId The folder ID
     * @param body Request body
     */
    suspend fun updateFolder(folderId: Long, body: UpdateFolderBody): FolderWithProjects {
        val info = OperationInfo(
            service = "Folders",
            operation = "UpdateFolder",
            resourceType = "folder",
            isMutation = true,
            projectId = null,
            resourceId = folderId,
        )
        return request(info, {
            httpPut("/stacks/${folderId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<FolderWithProjects>(body)
        }
    }

    /**
     * Delete a folder and unpin its projects from the home screen.
     * @param folderId The folder ID
     */
    suspend fun deleteFolder(folderId: Long): Unit {
        val info = OperationInfo(
            service = "Folders",
            operation = "DeleteFolder",
            resourceType = "folder",
            isMutation = true,
            projectId = null,
            resourceId = folderId,
        )
        request(info, {
            httpDelete("/stacks/${folderId}", operationName = info.operation)
        }) { Unit }
    }
}
