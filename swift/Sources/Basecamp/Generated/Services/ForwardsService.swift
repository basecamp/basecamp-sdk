// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListRepliesForwardOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListForwardOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class ForwardsService: BaseService, @unchecked Sendable {
    public func createReply(forwardId: Int, req: CreateForwardReplyRequest) async throws -> ForwardReply {
        return try await request(
            OperationInfo(service: "Forwards", operation: "CreateForwardReply", resourceType: "forward_reply", isMutation: true, resourceId: forwardId),
            method: "POST",
            path: "/inbox_forwards/\(forwardId)/replies.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateForwardReply")
        )
    }

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
            OperationInfo(service: "Forwards", operation: "GetForwardReply", resourceType: "forward_reply", isMutation: false, resourceId: forwardId),
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
        return try await requestPaginated(
            OperationInfo(service: "Forwards", operation: "ListForwardReplies", resourceType: "forward_reply", isMutation: false, resourceId: forwardId),
            path: "/inbox_forwards/\(forwardId)/replies.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListForwardReplies")
        )
    }

    public func list(inboxId: Int, options: ListForwardOptions? = nil) async throws -> ListResult<Forward> {
        return try await requestPaginated(
            OperationInfo(service: "Forwards", operation: "ListForwards", resourceType: "forward", isMutation: false, resourceId: inboxId),
            path: "/inboxes/\(inboxId)/forwards.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListForwards")
        )
    }
}
