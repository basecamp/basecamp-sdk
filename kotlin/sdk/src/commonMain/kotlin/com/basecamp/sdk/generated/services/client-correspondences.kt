package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for ClientCorrespondences operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class ClientCorrespondencesService(client: AccountClient) : BaseService(client) {

    /**
     * List all client correspondences in a project
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(options: PaginationOptions? = null): ListResult<ClientCorrespondence> {
        val info = OperationInfo(
            service = "ClientCorrespondences",
            operation = "ListClientCorrespondences",
            resourceType = "client_correspondence",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/client/correspondences.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<ClientCorrespondence>>(body)
        }
    }

    /**
     * Get a single client correspondence by id
     * @param correspondenceId The correspondence ID
     */
    suspend fun get(correspondenceId: Long): ClientCorrespondence {
        val info = OperationInfo(
            service = "ClientCorrespondences",
            operation = "GetClientCorrespondence",
            resourceType = "client_correspondence",
            isMutation = false,
            projectId = null,
            resourceId = correspondenceId,
        )
        return request(info, {
            httpGet("/client/correspondences/${correspondenceId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<ClientCorrespondence>(body)
        }
    }
}
