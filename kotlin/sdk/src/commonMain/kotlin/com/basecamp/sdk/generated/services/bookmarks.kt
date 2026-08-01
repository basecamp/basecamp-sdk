package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Bookmarks operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class BookmarksService(client: AccountClient) : BaseService(client) {

    /**
     * List the current user's bookmarks, most recently bookmarked first (paginated).
     * @param options Optional query parameters and pagination control
     */
    suspend fun listMyBookmarks(options: ListMyBookmarksOptions? = null): ListResult<Bookmark> {
        val info = OperationInfo(
            service = "Bookmarks",
            operation = "ListMyBookmarks",
            resourceType = "bookmark",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/my/bookmarks.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Bookmark>>(body)
        }
    }

    /**
     * Report whether the current user has bookmarked the recording.
     * @param recordingId The recording ID
     */
    suspend fun getBookmark(recordingId: Long): BookmarkStatus {
        val info = OperationInfo(
            service = "Bookmarks",
            operation = "GetBookmark",
            resourceType = "bookmark",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpGet("/recordings/${recordingId}/bookmark.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<BookmarkStatus>(body)
        }
    }

    /**
     * Bookmark a recording for the current user.
     * @param recordingId The recording ID
     */
    suspend fun createBookmark(recordingId: Long): Bookmark {
        val info = OperationInfo(
            service = "Bookmarks",
            operation = "CreateBookmark",
            resourceType = "bookmark",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpPost("/recordings/${recordingId}/bookmark.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Bookmark>(body)
        }
    }

    /**
     * Remove the current user's bookmark from a recording.
     * @param recordingId The recording ID
     */
    suspend fun deleteBookmark(recordingId: Long): Unit {
        val info = OperationInfo(
            service = "Bookmarks",
            operation = "DeleteBookmark",
            resourceType = "bookmark",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        request(info, {
            httpDelete("/recordings/${recordingId}/bookmark.json", operationName = info.operation)
        }) { Unit }
    }
}
