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
     * @param vaultId The vault ID
     */
    suspend fun get(vaultId: Long): Vault {
        val info = OperationInfo(
            service = "Vaults",
            operation = "GetVault",
            resourceType = "vault",
            isMutation = false,
            projectId = null,
            resourceId = vaultId,
        )
        return request(info, {
            httpGet("/vaults/${vaultId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Vault>(body)
        }
    }

    /**
     * Update an existing vault
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun update(vaultId: Long, body: UpdateVaultBody): Vault {
        val info = OperationInfo(
            service = "Vaults",
            operation = "UpdateVault",
            resourceType = "vault",
            isMutation = true,
            projectId = null,
            resourceId = vaultId,
        )
        return request(info, {
            httpPut("/vaults/${vaultId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Vault>(body)
        }
    }

    /**
     * List vaults (subfolders) in a vault
     * @param vaultId The vault ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(vaultId: Long, options: PaginationOptions? = null): ListResult<Vault> {
        val info = OperationInfo(
            service = "Vaults",
            operation = "ListVaults",
            resourceType = "vault",
            isMutation = false,
            projectId = null,
            resourceId = vaultId,
        )
        return requestPaginated(info, options, {
            httpGet("/vaults/${vaultId}/vaults.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Vault>>(body)
        }
    }

    /**
     * Create a new vault (subfolder) in a vault
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun create(vaultId: Long, body: CreateVaultBody): Vault {
        val info = OperationInfo(
            service = "Vaults",
            operation = "CreateVault",
            resourceType = "vault",
            isMutation = true,
            projectId = null,
            resourceId = vaultId,
        )
        return request(info, {
            httpPost("/vaults/${vaultId}/vaults.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Vault>(body)
        }
    }
}
