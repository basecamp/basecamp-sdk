// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListMyBookmarksBookmarkOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class BookmarksService: BaseService, @unchecked Sendable {
    public func createBookmark(recordingId: Int) async throws -> Bookmark {
        return try await request(
            OperationInfo(service: "Bookmarks", operation: "CreateBookmark", resourceType: "bookmark", isMutation: true, resourceId: recordingId),
            method: "POST",
            path: "/recordings/\(recordingId)/bookmark.json",
            retryConfig: Metadata.retryConfig(for: "CreateBookmark")
        )
    }

    public func deleteBookmark(recordingId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Bookmarks", operation: "DeleteBookmark", resourceType: "bookmark", isMutation: true, resourceId: recordingId),
            method: "DELETE",
            path: "/recordings/\(recordingId)/bookmark.json",
            retryConfig: Metadata.retryConfig(for: "DeleteBookmark")
        )
    }

    public func getBookmark(recordingId: Int) async throws -> BookmarkStatus {
        return try await request(
            OperationInfo(service: "Bookmarks", operation: "GetBookmark", resourceType: "bookmark", isMutation: false, resourceId: recordingId),
            method: "GET",
            path: "/recordings/\(recordingId)/bookmark.json",
            retryConfig: Metadata.retryConfig(for: "GetBookmark")
        )
    }

    public func listMyBookmarks(options: ListMyBookmarksBookmarkOptions? = nil) async throws -> ListResult<Bookmark> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Bookmarks", operation: "ListMyBookmarks", resourceType: "bookmark", isMutation: false),
            path: "/my/bookmarks.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListMyBookmarks")
        )
    }
}
