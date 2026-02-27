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
     * @param uploadId The upload ID
     */
    suspend fun get(uploadId: Long): Upload {
        val info = OperationInfo(
            service = "Uploads",
            operation = "GetUpload",
            resourceType = "upload",
            isMutation = false,
            projectId = null,
            resourceId = uploadId,
        )
        return request(info, {
            httpGet("/uploads/${uploadId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Upload>(body)
        }
    }

    /**
     * Update an existing upload
     * @param uploadId The upload ID
     * @param body Request body
     */
    suspend fun update(uploadId: Long, body: UpdateUploadBody): Upload {
        val info = OperationInfo(
            service = "Uploads",
            operation = "UpdateUpload",
            resourceType = "upload",
            isMutation = true,
            projectId = null,
            resourceId = uploadId,
        )
        return request(info, {
            httpPut("/uploads/${uploadId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.baseName?.let { put("base_name", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Upload>(body)
        }
    }

    /**
     * List versions of an upload
     * @param uploadId The upload ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listVersions(uploadId: Long, options: PaginationOptions? = null): ListResult<Upload> {
        val info = OperationInfo(
            service = "Uploads",
            operation = "ListUploadVersions",
            resourceType = "upload_version",
            isMutation = false,
            projectId = null,
            resourceId = uploadId,
        )
        return requestPaginated(info, options, {
            httpGet("/uploads/${uploadId}/versions.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Upload>>(body)
        }
    }

    /**
     * List uploads in a vault
     * @param vaultId The vault ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(vaultId: Long, options: PaginationOptions? = null): ListResult<Upload> {
        val info = OperationInfo(
            service = "Uploads",
            operation = "ListUploads",
            resourceType = "upload",
            isMutation = false,
            projectId = null,
            resourceId = vaultId,
        )
        return requestPaginated(info, options, {
            httpGet("/vaults/${vaultId}/uploads.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Upload>>(body)
        }
    }

    /**
     * Create a new upload in a vault
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun create(vaultId: Long, body: CreateUploadBody): Upload {
        val info = OperationInfo(
            service = "Uploads",
            operation = "CreateUpload",
            resourceType = "upload",
            isMutation = true,
            projectId = null,
            resourceId = vaultId,
        )
        return request(info, {
            httpPost("/vaults/${vaultId}/uploads.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("attachable_sgid", kotlinx.serialization.json.JsonPrimitive(body.attachableSgid))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.baseName?.let { put("base_name", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Upload>(body)
        }
    }
}
