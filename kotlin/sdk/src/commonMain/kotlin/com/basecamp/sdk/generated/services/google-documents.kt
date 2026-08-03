package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for GoogleDocuments operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class GoogleDocumentsService(client: AccountClient) : BaseService(client) {

    /**
     * Create a new Google document in a vault.
     * @param bucketId The bucket ID
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun createGoogleDocument(bucketId: Long, vaultId: Long, body: CreateGoogleDocumentBody): JsonElement {
        val info = OperationInfo(
            service = "GoogleDocuments",
            operation = "CreateGoogleDocument",
            resourceType = "google_document",
            isMutation = true,
            projectId = bucketId,
            resourceId = vaultId,
        )
        return request(info, {
            httpPost("/buckets/${bucketId}/vaults/${vaultId}/google_documents.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("url", kotlinx.serialization.json.JsonPrimitive(body.url))
                put("document_type", kotlinx.serialization.json.JsonPrimitive(body.documentType))
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.status?.let { put("status", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.visibleToClients?.let { put("visible_to_clients", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get a single Google document by id
     * @param googleDocumentId The google document ID
     */
    suspend fun googleDocument(googleDocumentId: Long): JsonElement {
        val info = OperationInfo(
            service = "GoogleDocuments",
            operation = "GetGoogleDocument",
            resourceType = "google_document",
            isMutation = false,
            projectId = null,
            resourceId = googleDocumentId,
        )
        return request(info, {
            httpGet("/google_documents/${googleDocumentId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Replace a Google document with a new complete representation.
     * @param googleDocumentId The google document ID
     * @param body Request body
     */
    suspend fun updateGoogleDocument(googleDocumentId: Long, body: UpdateGoogleDocumentBody): JsonElement {
        val info = OperationInfo(
            service = "GoogleDocuments",
            operation = "UpdateGoogleDocument",
            resourceType = "google_document",
            isMutation = true,
            projectId = null,
            resourceId = googleDocumentId,
        )
        return request(info, {
            httpPut("/google_documents/${googleDocumentId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("url", kotlinx.serialization.json.JsonPrimitive(body.url))
                put("document_type", kotlinx.serialization.json.JsonPrimitive(body.documentType))
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.status?.let { put("status", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
