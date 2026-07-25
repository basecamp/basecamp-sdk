package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Everything operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class EverythingService(client: AccountClient) : BaseService(client) {

    /**
     * Get every boost across all accessible projects, newest-first (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingBoosts(options: GetEverythingBoostsOptions? = null): ListResult<Boost> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingBoosts",
            resourceType = "everything_boost",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/boosts.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Boost>>(body)
        }
    }

    /**
     * Get every overdue card across all accessible projects, oldest-due-date-first.
     */
    suspend fun everythingOverdueCards(): List<Card> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingOverdueCards",
            resourceType = "everything_overdue_card",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/cards/overdue.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Card>>(body)
        }
    }

    /**
     * Get every automatic check-in answer across all accessible projects,
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingCheckins(options: GetEverythingCheckinsOptions? = null): ListResult<Recording> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingCheckins",
            resourceType = "everything_checkin",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/checkins.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Recording>>(body)
        }
    }

    /**
     * Get every comment across all accessible projects, newest-first (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingComments(options: GetEverythingCommentsOptions? = null): ListResult<Recording> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingComments",
            resourceType = "everything_comment",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/comments.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Recording>>(body)
        }
    }

    /**
     * Get every file recording across all accessible projects, newest-first
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingFiles(options: GetEverythingFilesOptions? = null): ListResult<EverythingFile> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingFiles",
            resourceType = "everything_file",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "kind" to options?.kind,
            "people_ids[]" to options?.peopleIds,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/files.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<EverythingFile>>(body)
        }
    }

    /**
     * Get every inbox forward across all accessible projects, newest-first
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingForwards(options: GetEverythingForwardsOptions? = null): ListResult<Recording> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingForwards",
            resourceType = "everything_forward",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/forwards.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Recording>>(body)
        }
    }

    /**
     * Get every message across all accessible projects, newest-first (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingMessages(options: GetEverythingMessagesOptions? = null): ListResult<Recording> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingMessages",
            resourceType = "everything_message",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/messages.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Recording>>(body)
        }
    }

    /**
     * Get every overdue to-do across all accessible projects, oldest-due-date-first.
     */
    suspend fun everythingOverdueTodos(): List<Todo> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingOverdueTodos",
            resourceType = "everything_overdue_todo",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/todos/overdue.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Todo>>(body)
        }
    }
}
