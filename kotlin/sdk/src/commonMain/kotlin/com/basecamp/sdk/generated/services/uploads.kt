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
open class UploadsService(client: AccountClient) : BaseService(client) {

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
    suspend fun listVersions(uploadId: Long, options: PaginationOptions? = null): ListResult<JsonElement> {
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
            json.decodeFromString<List<JsonElement>>(body)
        }
    }

    /**
     * Replace an upload's file with a new version
     * @param uploadId The upload ID
     * @param body Request body
     */
    suspend fun createVersion(uploadId: Long, body: CreateUploadVersionBody): Upload {
        val info = OperationInfo(
            service = "Uploads",
            operation = "CreateUploadVersion",
            resourceType = "upload_version",
            isMutation = true,
            projectId = null,
            resourceId = uploadId,
        )
        return request(info, {
            httpPost("/uploads/${uploadId}/versions.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("attachable_sgid", kotlinx.serialization.json.JsonPrimitive(body.attachableSgid))
                body.baseName?.let { put("base_name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.notify?.let { put("notify", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Upload>(body)
        }
    }

    /**
     * List uploads in a vault
     * @param vaultId The vault ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(vaultId: Long, options: ListUploadsOptions): ListResult<Upload> {
        val info = OperationInfo(
            service = "Uploads",
            operation = "ListUploads",
            resourceType = "upload",
            isMutation = false,
            projectId = null,
            resourceId = vaultId,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/vaults/${vaultId}/uploads.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Upload>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [ListUploadsOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [list] needs an expected type to disambiguate.
     */
    suspend fun list(vaultId: Long, options: PaginationOptions? = null): ListResult<Upload> =
        list(vaultId, ListUploadsOptions(maxItems = options?.maxItems, page = options?.page))

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
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.visibleToClients?.let { put("visible_to_clients", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Upload>(body)
        }
    }
}
