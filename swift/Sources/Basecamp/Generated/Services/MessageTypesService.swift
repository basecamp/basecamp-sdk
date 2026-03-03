// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListMessageTypeOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class MessageTypesService: BaseService, @unchecked Sendable {
    public func create(req: CreateMessageTypeRequest) async throws -> MessageType {
        return try await request(
            OperationInfo(service: "MessageTypes", operation: "CreateMessageType", resourceType: "message_type", isMutation: true),
            method: "POST",
            path: "/categories.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateMessageType")
        )
    }

    public func delete(typeId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "MessageTypes", operation: "DeleteMessageType", resourceType: "message_type", isMutation: true, resourceId: typeId),
            method: "DELETE",
            path: "/categories/\(typeId)",
            retryConfig: Metadata.retryConfig(for: "DeleteMessageType")
        )
    }

    public func get(typeId: Int) async throws -> MessageType {
        return try await request(
            OperationInfo(service: "MessageTypes", operation: "GetMessageType", resourceType: "message_type", isMutation: false, resourceId: typeId),
            method: "GET",
            path: "/categories/\(typeId)",
            retryConfig: Metadata.retryConfig(for: "GetMessageType")
        )
    }

    public func list(options: ListMessageTypeOptions? = nil) async throws -> ListResult<MessageType> {
        return try await requestPaginated(
            OperationInfo(service: "MessageTypes", operation: "ListMessageTypes", resourceType: "message_type", isMutation: false),
            path: "/categories.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListMessageTypes")
        )
    }

    public func update(typeId: Int, req: UpdateMessageTypeRequest) async throws -> MessageType {
        return try await request(
            OperationInfo(service: "MessageTypes", operation: "UpdateMessageType", resourceType: "message_type", isMutation: true, resourceId: typeId),
            method: "PUT",
            path: "/categories/\(typeId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateMessageType")
        )
    }
}
