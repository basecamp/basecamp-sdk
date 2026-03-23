package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Account operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class AccountService(client: AccountClient) : BaseService(client) {

    /**
     * Get the account for the current access token
     */
    suspend fun account(): JsonElement {
        val info = OperationInfo(
            service = "Account",
            operation = "GetAccount",
            resourceType = "account",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/account.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Upload or replace the account logo via multipart form upload.
     * @param data Binary file data to upload
     * @param contentType MIME type of the file
     */
    suspend fun updateAccountLogo(data: ByteArray, contentType: String): Unit {
        val info = OperationInfo(
            service = "Account",
            operation = "UpdateAccountLogo",
            resourceType = "account_logo",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        request(info, {
            httpPut("/account/logo.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Remove the account logo. Only administrators and account owners can use this endpoint.
     */
    suspend fun removeAccountLogo(): Unit {
        val info = OperationInfo(
            service = "Account",
            operation = "RemoveAccountLogo",
            resourceType = "resource",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        request(info, {
            httpDelete("/account/logo.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Rename the current account. Only account owners can use this endpoint.
     * @param body Request body
     */
    suspend fun updateAccountName(body: UpdateAccountNameBody): JsonElement {
        val info = OperationInfo(
            service = "Account",
            operation = "UpdateAccountName",
            resourceType = "account_name",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPut("/account/name.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
