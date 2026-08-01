package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for ClientApprovals operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class ClientApprovalsService(client: AccountClient) : BaseService(client) {

    /**
     * List all client approvals in a project
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(options: ListClientApprovalsOptions? = null): ListResult<ClientApproval> {
        val info = OperationInfo(
            service = "ClientApprovals",
            operation = "ListClientApprovals",
            resourceType = "client_approval",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "sort" to options?.sort,
            "direction" to options?.direction,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/client/approvals.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<ClientApproval>>(body)
        }
    }

    /**
     * Get a single client approval by id
     * @param approvalId The approval ID
     */
    suspend fun get(approvalId: Long): ClientApproval {
        val info = OperationInfo(
            service = "ClientApprovals",
            operation = "GetClientApproval",
            resourceType = "client_approval",
            isMutation = false,
            projectId = null,
            resourceId = approvalId,
        )
        return request(info, {
            httpGet("/client/approvals/${approvalId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<ClientApproval>(body)
        }
    }
}
