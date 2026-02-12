// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListMessageOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class MessagesService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, boardId: Int, req: CreateMessageRequest) async throws -> Message {
        return try await request(
            OperationInfo(service: "Messages", operation: "CreateMessage", resourceType: "message", isMutation: true, projectId: projectId, resourceId: boardId),
            method: "POST",
            path: "/buckets/\(projectId)/message_boards/\(boardId)/messages.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateMessage")
        )
    }

    public func get(projectId: Int, messageId: Int) async throws -> Message {
        return try await request(
            OperationInfo(service: "Messages", operation: "GetMessage", resourceType: "message", isMutation: false, projectId: projectId, resourceId: messageId),
            method: "GET",
            path: "/buckets/\(projectId)/messages/\(messageId)",
            retryConfig: Metadata.retryConfig(for: "GetMessage")
        )
    }

    public func list(projectId: Int, boardId: Int, options: ListMessageOptions? = nil) async throws -> ListResult<Message> {
        return try await requestPaginated(
            OperationInfo(service: "Messages", operation: "ListMessages", resourceType: "message", isMutation: false, projectId: projectId, resourceId: boardId),
            path: "/buckets/\(projectId)/message_boards/\(boardId)/messages.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListMessages")
        )
    }

    public func pin(projectId: Int, messageId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Messages", operation: "PinMessage", resourceType: "message", isMutation: true, projectId: projectId, resourceId: messageId),
            method: "POST",
            path: "/buckets/\(projectId)/recordings/\(messageId)/pin.json",
            retryConfig: Metadata.retryConfig(for: "PinMessage")
        )
    }

    public func unpin(projectId: Int, messageId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Messages", operation: "UnpinMessage", resourceType: "message", isMutation: true, projectId: projectId, resourceId: messageId),
            method: "DELETE",
            path: "/buckets/\(projectId)/recordings/\(messageId)/pin.json",
            retryConfig: Metadata.retryConfig(for: "UnpinMessage")
        )
    }

    public func update(projectId: Int, messageId: Int, req: UpdateMessageRequest) async throws -> Message {
        return try await request(
            OperationInfo(service: "Messages", operation: "UpdateMessage", resourceType: "message", isMutation: true, projectId: projectId, resourceId: messageId),
            method: "PUT",
            path: "/buckets/\(projectId)/messages/\(messageId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateMessage")
        )
    }
}
