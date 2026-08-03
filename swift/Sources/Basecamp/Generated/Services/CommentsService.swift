// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListCommentOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class CommentsService: BaseService, @unchecked Sendable {
    public func create(recordingId: Int, req: CreateCommentRequest) async throws -> Comment {
        return try await request(
            OperationInfo(service: "Comments", operation: "CreateComment", resourceType: "comment", isMutation: true, resourceId: recordingId),
            method: "POST",
            path: "/recordings/\(recordingId)/comments.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateComment")
        )
    }

    public func get(commentId: Int) async throws -> Comment {
        return try await request(
            OperationInfo(service: "Comments", operation: "GetComment", resourceType: "comment", isMutation: false, resourceId: commentId),
            method: "GET",
            path: "/comments/\(commentId)",
            retryConfig: Metadata.retryConfig(for: "GetComment")
        )
    }

    public func list(recordingId: Int, options: ListCommentOptions? = nil) async throws -> ListResult<Comment> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Comments", operation: "ListComments", resourceType: "comment", isMutation: false, resourceId: recordingId),
            path: "/recordings/\(recordingId)/comments.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListComments")
        )
    }

    public func update(commentId: Int, req: UpdateCommentRequest) async throws -> Comment {
        return try await request(
            OperationInfo(service: "Comments", operation: "UpdateComment", resourceType: "comment", isMutation: true, resourceId: commentId),
            method: "PUT",
            path: "/comments/\(commentId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateComment")
        )
    }
}
