package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Vaults operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class VaultsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a single vault by id
     * @param projectId The project ID
     * @param vaultId The vault ID
     */
    suspend fun get(projectId: Long, vaultId: Long): Vault {
        val info = OperationInfo(
            service = "Vaults",
            operation = "GetVault",
            resourceType = "vault",
            isMutation = false,
            projectId = projectId,
            resourceId = vaultId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/vaults/${vaultId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Vault>(body)
        }
    }

    /**
     * Update an existing vault
     * @param projectId The project ID
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun update(projectId: Long, vaultId: Long, body: UpdateVaultBody): Vault {
        val info = OperationInfo(
            service = "Vaults",
            operation = "UpdateVault",
            resourceType = "vault",
            isMutation = true,
            projectId = projectId,
            resourceId = vaultId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/vaults/${vaultId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Vault>(body)
        }
    }

    /**
     * List vaults (subfolders) in a vault
     * @param projectId The project ID
     * @param vaultId The vault ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(projectId: Long, vaultId: Long, options: PaginationOptions? = null): ListResult<Vault> {
        val info = OperationInfo(
            service = "Vaults",
            operation = "ListVaults",
            resourceType = "vault",
            isMutation = false,
            projectId = projectId,
            resourceId = vaultId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/vaults/${vaultId}/vaults.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Vault>>(body)
        }
    }

    /**
     * Create a new vault (subfolder) in a vault
     * @param projectId The project ID
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun create(projectId: Long, vaultId: Long, body: CreateVaultBody): Vault {
        val info = OperationInfo(
            service = "Vaults",
            operation = "CreateVault",
            resourceType = "vault",
            isMutation = true,
            projectId = projectId,
            resourceId = vaultId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/vaults/${vaultId}/vaults.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Vault>(body)
        }
    }
}
