package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Documents operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class DocumentsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a single document by id
     * @param documentId The document ID
     */
    suspend fun get(documentId: Long): Document {
        val info = OperationInfo(
            service = "Documents",
            operation = "GetDocument",
            resourceType = "document",
            isMutation = false,
            projectId = null,
            resourceId = documentId,
        )
        return request(info, {
            httpGet("/documents/${documentId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Document>(body)
        }
    }

    /**
     * Update an existing document
     * @param documentId The document ID
     * @param body Request body
     */
    suspend fun update(documentId: Long, body: UpdateDocumentBody): Document {
        val info = OperationInfo(
            service = "Documents",
            operation = "UpdateDocument",
            resourceType = "document",
            isMutation = true,
            projectId = null,
            resourceId = documentId,
        )
        return request(info, {
            httpPut("/documents/${documentId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.content?.let { put("content", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Document>(body)
        }
    }

    /**
     * List documents in a vault
     * @param vaultId The vault ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(vaultId: Long, options: PaginationOptions? = null): ListResult<Document> {
        val info = OperationInfo(
            service = "Documents",
            operation = "ListDocuments",
            resourceType = "document",
            isMutation = false,
            projectId = null,
            resourceId = vaultId,
        )
        return requestPaginated(info, options, {
            httpGet("/vaults/${vaultId}/documents.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Document>>(body)
        }
    }

    /**
     * Create a new document in a vault
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun create(vaultId: Long, body: CreateDocumentBody): Document {
        val info = OperationInfo(
            service = "Documents",
            operation = "CreateDocument",
            resourceType = "document",
            isMutation = true,
            projectId = null,
            resourceId = vaultId,
        )
        return request(info, {
            httpPost("/vaults/${vaultId}/documents.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
                body.content?.let { put("content", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.status?.let { put("status", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Document>(body)
        }
    }
}
