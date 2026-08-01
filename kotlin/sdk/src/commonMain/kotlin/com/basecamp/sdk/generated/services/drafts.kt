package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Drafts operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class DraftsService(client: AccountClient) : BaseService(client) {

    /**
     * List the current user's drafts across their active projects, most recently
     * @param options Optional query parameters and pagination control
     */
    suspend fun listMyDrafts(options: ListMyDraftsOptions? = null): ListResult<Draft> {
        val info = OperationInfo(
            service = "Drafts",
            operation = "ListMyDrafts",
            resourceType = "my_draft",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/my/drafts.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Draft>>(body)
        }
    }

    /**
     * Source-compatibility overload: accepts bare [PaginationOptions].
     *
     * Prefer [ListMyDraftsOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     */
    suspend fun listMyDrafts(options: PaginationOptions): ListResult<Draft> =
        listMyDrafts(ListMyDraftsOptions(maxItems = options.maxItems))
}
