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
     * Completed cards across all accessible projects, grouped by project (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingCompletedCards(options: GetEverythingCompletedCardsOptions? = null): ListResult<BucketCardsGroup> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingCompletedCards",
            resourceType = "everything_completed_card",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/cards/completed.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<BucketCardsGroup>>(body)
        }
    }

    /**
     * Open cards with no due date across all accessible projects, grouped by project (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingNoDueDateCards(options: GetEverythingNoDueDateCardsOptions? = null): ListResult<BucketCardsGroup> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingNoDueDateCards",
            resourceType = "everything_no_due_date_card",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/cards/no_due_date.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<BucketCardsGroup>>(body)
        }
    }

    /**
     * Cards parked in a project's "Not now" column across all accessible projects, grouped by project (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingNotNowCards(options: GetEverythingNotNowCardsOptions? = null): ListResult<BucketCardsGroup> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingNotNowCards",
            resourceType = "everything_not_now_card",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/cards/not_now.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<BucketCardsGroup>>(body)
        }
    }

    /**
     * Incomplete cards in active columns across all accessible projects, grouped by project (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingOpenCards(options: GetEverythingOpenCardsOptions? = null): ListResult<BucketCardsGroup> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingOpenCards",
            resourceType = "everything_open_card",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/cards/open.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<BucketCardsGroup>>(body)
        }
    }

    /**
     * Get every overdue card across all accessible projects, oldest-due-date-first.
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingOverdueCards(options: GetEverythingOverdueCardsOptions? = null): List<Card> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingOverdueCards",
            resourceType = "everything_overdue_card",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
        )
        return request(info, {
            httpGet("/cards/overdue.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Card>>(body)
        }
    }

    /**
     * Open, unassigned cards across all accessible projects, grouped by project (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingUnassignedCards(options: GetEverythingUnassignedCardsOptions? = null): ListResult<BucketCardsGroup> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingUnassignedCards",
            resourceType = "everything_unassigned_card",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/cards/unassigned.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<BucketCardsGroup>>(body)
        }
    }

    /**
     * Get every automatic check-in answer across all accessible projects, newest-first.
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
     * Get every file recording across all accessible projects, newest-first (paginated).
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
     * Get every inbox forward across all accessible projects, newest-first (paginated).
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
     * Completed to-dos across all accessible projects, grouped by project (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingCompletedTodos(options: GetEverythingCompletedTodosOptions? = null): ListResult<BucketTodosGroup> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingCompletedTodos",
            resourceType = "everything_completed_todo",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/todos/completed.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<BucketTodosGroup>>(body)
        }
    }

    /**
     * Open to-dos with no due date across all accessible projects, grouped by project (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingNoDueDateTodos(options: GetEverythingNoDueDateTodosOptions? = null): ListResult<BucketTodosGroup> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingNoDueDateTodos",
            resourceType = "everything_no_due_date_todo",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/todos/no_due_date.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<BucketTodosGroup>>(body)
        }
    }

    /**
     * Active, incomplete to-dos across all accessible projects, grouped by project (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingOpenTodos(options: GetEverythingOpenTodosOptions? = null): ListResult<BucketTodosGroup> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingOpenTodos",
            resourceType = "everything_open_todo",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/todos/open.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<BucketTodosGroup>>(body)
        }
    }

    /**
     * Get every overdue to-do across all accessible projects, oldest-due-date-first.
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingOverdueTodos(options: GetEverythingOverdueTodosOptions? = null): List<Todo> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingOverdueTodos",
            resourceType = "everything_overdue_todo",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
        )
        return request(info, {
            httpGet("/todos/overdue.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Todo>>(body)
        }
    }

    /**
     * Open, unassigned to-dos across all accessible projects, grouped by project (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun everythingUnassignedTodos(options: GetEverythingUnassignedTodosOptions? = null): ListResult<BucketTodosGroup> {
        val info = OperationInfo(
            service = "Everything",
            operation = "GetEverythingUnassignedTodos",
            resourceType = "everything_unassigned_todo",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "assignee_ids[]" to options?.assigneeIds,
            "due" to options?.due,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/todos/unassigned.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<BucketTodosGroup>>(body)
        }
    }
}
