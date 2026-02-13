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
     * @param projectId The project ID
     * @param documentId The document ID
     */
    suspend fun get(projectId: Long, documentId: Long): Document {
        val info = OperationInfo(
            service = "Documents",
            operation = "GetDocument",
            resourceType = "document",
            isMutation = false,
            projectId = projectId,
            resourceId = documentId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/documents/${documentId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Document>(body)
        }
    }

    /**
     * Update an existing document
     * @param projectId The project ID
     * @param documentId The document ID
     * @param body Request body
     */
    suspend fun update(projectId: Long, documentId: Long, body: UpdateDocumentBody): Document {
        val info = OperationInfo(
            service = "Documents",
            operation = "UpdateDocument",
            resourceType = "document",
            isMutation = true,
            projectId = projectId,
            resourceId = documentId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/documents/${documentId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.content?.let { put("content", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Document>(body)
        }
    }

    /**
     * List documents in a vault
     * @param projectId The project ID
     * @param vaultId The vault ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(projectId: Long, vaultId: Long, options: PaginationOptions? = null): ListResult<Document> {
        val info = OperationInfo(
            service = "Documents",
            operation = "ListDocuments",
            resourceType = "document",
            isMutation = false,
            projectId = projectId,
            resourceId = vaultId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/vaults/${vaultId}/documents.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Document>>(body)
        }
    }

    /**
     * Create a new document in a vault
     * @param projectId The project ID
     * @param vaultId The vault ID
     * @param body Request body
     */
    suspend fun create(projectId: Long, vaultId: Long, body: CreateDocumentBody): Document {
        val info = OperationInfo(
            service = "Documents",
            operation = "CreateDocument",
            resourceType = "document",
            isMutation = true,
            projectId = projectId,
            resourceId = vaultId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/vaults/${vaultId}/documents.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
                body.content?.let { put("content", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.status?.let { put("status", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Document>(body)
        }
    }
}
