package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Search operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class SearchService(client: AccountClient) : BaseService(client) {

    /**
     * Search for content across the account
     * @param query query
     * @param options Optional query parameters and pagination control
     */
    suspend fun search(query: String, options: SearchOptions? = null): ListResult<JsonElement> {
        val info = OperationInfo(
            service = "Search",
            operation = "Search",
            resourceType = "resource",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "query" to query,
            "sort" to options?.sort,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/search.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<JsonElement>>(body)
        }
    }

    /**
     * Get search metadata (available filter options)
     */
    suspend fun metadata(): JsonElement {
        val info = OperationInfo(
            service = "Search",
            operation = "GetSearchMetadata",
            resourceType = "search_metadata",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/searches/metadata.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
