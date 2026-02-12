package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Attachments operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class AttachmentsService(client: AccountClient) : BaseService(client) {

    /**
     * Create an attachment (upload a file for embedding)
     * @param data Binary file data to upload
     * @param contentType MIME type of the file
     * @param name name
     */
    suspend fun create(data: ByteArray, contentType: String, name: String): JsonElement {
        val info = OperationInfo(
            service = "Attachments",
            operation = "CreateAttachment",
            resourceType = "attachment",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "name" to name,
        )
        return request(info, {
            httpPostBinary("/attachments.json" + qs, data, contentType)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
