package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Uploads operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class UploadsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a single upload by id
     * @param projectId The project ID
     * @param uploadId The upload ID
     */
    suspend fun get(projectId: Long, uploadId: Long): Upload {
        val info = OperationInfo(
            service = "Uploads",
            operation = "GetUpload",
            resourceType = "upload",
            isMutation = false,
            projectId = projectId,
            resourceId = uploadId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/uploads/${uploadId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Upload>(body)
        }
    }

    /**
     * Update an existing upload
     * @param projectId The project ID
     * @param uploadId The upload ID
     * @param body Request body
     */
    suspend fun update(projectId: Long, uploadId: Long, body: UpdateUploadBody): Upload {
        val info = OperationInfo(
            service = "Uploads",
            operation = "UpdateUpload",
            resourceType = "upload",
            isMutation = true,
            projectId = projectId,
            resourceId = uploadId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/uploads/${uploadId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.baseName?.let { put("base_name", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Upload>(body)
        }
    }

    /**
     * List versions of an upload
     * @param projectId The project ID
     * @param uploadId The upload ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listVersions(projectId: Long, uploadId: Long, options: PaginationOptions? = null): ListResult<Upload> {
        val info = OperationInfo(
            service = "Uploads",
            operation = "ListUploadVersions",
            resourceType = "upload_version",
            isMutation = false,
            projectId = projectId,
            resourceId = uploadId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/uploads/${uploadId}/versions.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Upload>>(body)
        }
    }

    /**
     * List uploads in a vault
     * @param projectId The project ID
     * @param vaultId The vault ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(projectId: Long, vaultId: Long, options: PaginationOptions? = null): ListResult<Upload> {
        val info = OperationInfo(
            service = "Uploads",
            operation = "ListUploads",
            resourceType = "upload",
            isMutation = false,
            projectId = projectId,
            resourceId = vaultId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/vaults/${vaultId}/uploads.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Upload>>(body)
        }
    }

    /**
     * Create a new upload in a vault
     * @param projectId The project ID
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun create(projectId: Long, vaultId: Long, body: CreateUploadBody): Upload {
        val info = OperationInfo(
            service = "Uploads",
            operation = "CreateUpload",
            resourceType = "upload",
            isMutation = true,
            projectId = projectId,
            resourceId = vaultId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/vaults/${vaultId}/uploads.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("attachable_sgid", kotlinx.serialization.json.JsonPrimitive(body.attachableSgid))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.baseName?.let { put("base_name", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Upload>(body)
        }
    }
}
