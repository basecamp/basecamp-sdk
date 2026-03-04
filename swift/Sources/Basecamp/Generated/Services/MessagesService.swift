// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListMessageOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class MessagesService: BaseService, @unchecked Sendable {
    public func create(boardId: Int, req: CreateMessageRequest) async throws -> Message {
        return try await request(
            OperationInfo(service: "Messages", operation: "CreateMessage", resourceType: "message", isMutation: true, resourceId: boardId),
            method: "POST",
            path: "/message_boards/\(boardId)/messages.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateMessage")
        )
    }

    public func get(messageId: Int) async throws -> Message {
        return try await request(
            OperationInfo(service: "Messages", operation: "GetMessage", resourceType: "message", isMutation: false, resourceId: messageId),
            method: "GET",
            path: "/messages/\(messageId)",
            retryConfig: Metadata.retryConfig(for: "GetMessage")
        )
    }

    public func list(boardId: Int, options: ListMessageOptions? = nil) async throws -> ListResult<Message> {
        return try await requestPaginated(
            OperationInfo(service: "Messages", operation: "ListMessages", resourceType: "message", isMutation: false, resourceId: boardId),
            path: "/message_boards/\(boardId)/messages.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListMessages")
        )
    }

    public func pin(messageId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Messages", operation: "PinMessage", resourceType: "message", isMutation: true, resourceId: messageId),
            method: "POST",
            path: "/recordings/\(messageId)/pin.json",
            retryConfig: Metadata.retryConfig(for: "PinMessage")
        )
    }

    public func unpin(messageId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Messages", operation: "UnpinMessage", resourceType: "message", isMutation: true, resourceId: messageId),
            method: "DELETE",
            path: "/recordings/\(messageId)/pin.json",
            retryConfig: Metadata.retryConfig(for: "UnpinMessage")
        )
    }

    public func update(messageId: Int, req: UpdateMessageRequest) async throws -> Message {
        return try await request(
            OperationInfo(service: "Messages", operation: "UpdateMessage", resourceType: "message", isMutation: true, resourceId: messageId),
            method: "PUT",
            path: "/messages/\(messageId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateMessage")
        )
    }
}
