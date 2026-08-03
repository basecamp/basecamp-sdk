// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListRepliesForwardOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct ListForwardOptions: Sendable {
    /// created_at|updated_at
    public var sort: String?
    /// asc|desc
    public var direction: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        sort: String? = nil,
        direction: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.sort = sort
        self.direction = direction
        self.page = page
        self.maxItems = maxItems
    }
}


public final class ForwardsService: BaseService, @unchecked Sendable {
    public func get(forwardId: Int) async throws -> Forward {
        return try await request(
            OperationInfo(service: "Forwards", operation: "GetForward", resourceType: "forward", isMutation: false, resourceId: forwardId),
            method: "GET",
            path: "/inbox_forwards/\(forwardId)",
            retryConfig: Metadata.retryConfig(for: "GetForward")
        )
    }

    public func getReply(forwardId: Int, replyId: Int) async throws -> ForwardReply {
        return try await request(
            OperationInfo(service: "Forwards", operation: "GetForwardReply", resourceType: "forward_reply", isMutation: false, resourceId: replyId),
            method: "GET",
            path: "/inbox_forwards/\(forwardId)/replies/\(replyId)",
            retryConfig: Metadata.retryConfig(for: "GetForwardReply")
        )
    }

    public func getInbox(inboxId: Int) async throws -> Inbox {
        return try await request(
            OperationInfo(service: "Forwards", operation: "GetInbox", resourceType: "inbox", isMutation: false, resourceId: inboxId),
            method: "GET",
            path: "/inboxes/\(inboxId)",
            retryConfig: Metadata.retryConfig(for: "GetInbox")
        )
    }

    public func listReplies(forwardId: Int, options: ListRepliesForwardOptions? = nil) async throws -> ListResult<ForwardReply> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Forwards", operation: "ListForwardReplies", resourceType: "forward_reply", isMutation: false, resourceId: forwardId),
            path: "/inbox_forwards/\(forwardId)/replies.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListForwardReplies")
        )
    }

    public func list(inboxId: Int, options: ListForwardOptions? = nil) async throws -> ListResult<Forward> {
        var queryItems: [URLQueryItem] = []
        if let sort = options?.sort {
            queryItems.append(URLQueryItem(name: "sort", value: sort))
        }
        if let direction = options?.direction {
            queryItems.append(URLQueryItem(name: "direction", value: direction))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Forwards", operation: "ListForwards", resourceType: "forward", isMutation: false, resourceId: inboxId),
            path: "/inboxes/\(inboxId)/inbox_forwards.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListForwards")
        )
    }
}
