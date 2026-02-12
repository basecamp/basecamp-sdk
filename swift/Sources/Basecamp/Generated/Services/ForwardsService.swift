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
    public func createReply(projectId: Int, forwardId: Int, req: CreateForwardReplyRequest) async throws -> ForwardReply {
        return try await request(
            OperationInfo(service: "Forwards", operation: "CreateForwardReply", resourceType: "forward_reply", isMutation: true, projectId: projectId, resourceId: forwardId),
            method: "POST",
            path: "/buckets/\(projectId)/inbox_forwards/\(forwardId)/replies.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateForwardReply")
        )
    }

    public func get(projectId: Int, forwardId: Int) async throws -> Forward {
        return try await request(
            OperationInfo(service: "Forwards", operation: "GetForward", resourceType: "forward", isMutation: false, projectId: projectId, resourceId: forwardId),
            method: "GET",
            path: "/buckets/\(projectId)/inbox_forwards/\(forwardId)",
            retryConfig: Metadata.retryConfig(for: "GetForward")
        )
    }

    public func getReply(projectId: Int, forwardId: Int, replyId: Int) async throws -> ForwardReply {
        return try await request(
            OperationInfo(service: "Forwards", operation: "GetForwardReply", resourceType: "forward_reply", isMutation: false, projectId: projectId, resourceId: forwardId),
            method: "GET",
            path: "/buckets/\(projectId)/inbox_forwards/\(forwardId)/replies/\(replyId)",
            retryConfig: Metadata.retryConfig(for: "GetForwardReply")
        )
    }

    public func getInbox(projectId: Int, inboxId: Int) async throws -> Inbox {
        return try await request(
            OperationInfo(service: "Forwards", operation: "GetInbox", resourceType: "inbox", isMutation: false, projectId: projectId, resourceId: inboxId),
            method: "GET",
            path: "/buckets/\(projectId)/inboxes/\(inboxId)",
            retryConfig: Metadata.retryConfig(for: "GetInbox")
        )
    }

    public func listReplies(projectId: Int, forwardId: Int, options: ListRepliesForwardOptions? = nil) async throws -> ListResult<ForwardReply> {
        return try await requestPaginated(
            OperationInfo(service: "Forwards", operation: "ListForwardReplies", resourceType: "forward_reply", isMutation: false, projectId: projectId, resourceId: forwardId),
            path: "/buckets/\(projectId)/inbox_forwards/\(forwardId)/replies.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListForwardReplies")
        )
    }

    public func list(projectId: Int, inboxId: Int, options: ListForwardOptions? = nil) async throws -> ListResult<Forward> {
        return try await requestPaginated(
            OperationInfo(service: "Forwards", operation: "ListForwards", resourceType: "forward", isMutation: false, projectId: projectId, resourceId: inboxId),
            path: "/buckets/\(projectId)/inboxes/\(inboxId)/forwards.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListForwards")
        )
    }
}
