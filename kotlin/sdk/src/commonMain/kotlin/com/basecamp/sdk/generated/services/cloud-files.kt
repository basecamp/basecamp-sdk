package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for CloudFiles operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class CloudFilesService(client: AccountClient) : BaseService(client) {

    /**
     * Create a new cloud file in a vault.
     * @param bucketId The bucket ID
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun createCloudFile(bucketId: Long, vaultId: Long, body: CreateCloudFileBody): JsonElement {
        val info = OperationInfo(
            service = "CloudFiles",
            operation = "CreateCloudFile",
            resourceType = "cloud_file",
            isMutation = true,
            projectId = bucketId,
            resourceId = vaultId,
        )
        return request(info, {
            httpPost("/buckets/${bucketId}/vaults/${vaultId}/cloud_files.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("url", kotlinx.serialization.json.JsonPrimitive(body.url))
                put("service", kotlinx.serialization.json.JsonPrimitive(body.service))
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.visibleToClients?.let { put("visible_to_clients", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get a single cloud file by id
     * @param cloudFileId The cloud file ID
     */
    suspend fun cloudFile(cloudFileId: Long): JsonElement {
        val info = OperationInfo(
            service = "CloudFiles",
            operation = "GetCloudFile",
            resourceType = "cloud_file",
            isMutation = false,
            projectId = null,
            resourceId = cloudFileId,
        )
        return request(info, {
            httpGet("/cloud_files/${cloudFileId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Replace a cloud file with a new complete representation.
     * @param cloudFileId The cloud file ID
     * @param body Request body
     */
    suspend fun updateCloudFile(cloudFileId: Long, body: UpdateCloudFileBody): JsonElement {
        val info = OperationInfo(
            service = "CloudFiles",
            operation = "UpdateCloudFile",
            resourceType = "cloud_file",
            isMutation = true,
            projectId = null,
            resourceId = cloudFileId,
        )
        return request(info, {
            httpPut("/cloud_files/${cloudFileId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("url", kotlinx.serialization.json.JsonPrimitive(body.url))
                put("service", kotlinx.serialization.json.JsonPrimitive(body.service))
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
