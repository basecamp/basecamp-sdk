// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListMessageTypeOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class MessageTypesService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, req: CreateMessageTypeRequest) async throws -> MessageType {
        return try await request(
            OperationInfo(service: "MessageTypes", operation: "CreateMessageType", resourceType: "message_type", isMutation: true, projectId: projectId),
            method: "POST",
            path: "/buckets/\(projectId)/categories.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateMessageType")
        )
    }

    public func delete(projectId: Int, typeId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "MessageTypes", operation: "DeleteMessageType", resourceType: "message_type", isMutation: true, projectId: projectId, resourceId: typeId),
            method: "DELETE",
            path: "/buckets/\(projectId)/categories/\(typeId)",
            retryConfig: Metadata.retryConfig(for: "DeleteMessageType")
        )
    }

    public func get(projectId: Int, typeId: Int) async throws -> MessageType {
        return try await request(
            OperationInfo(service: "MessageTypes", operation: "GetMessageType", resourceType: "message_type", isMutation: false, projectId: projectId, resourceId: typeId),
            method: "GET",
            path: "/buckets/\(projectId)/categories/\(typeId)",
            retryConfig: Metadata.retryConfig(for: "GetMessageType")
        )
    }

    public func list(projectId: Int, options: ListMessageTypeOptions? = nil) async throws -> ListResult<MessageType> {
        return try await requestPaginated(
            OperationInfo(service: "MessageTypes", operation: "ListMessageTypes", resourceType: "message_type", isMutation: false, projectId: projectId),
            path: "/buckets/\(projectId)/categories.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListMessageTypes")
        )
    }

    public func update(projectId: Int, typeId: Int, req: UpdateMessageTypeRequest) async throws -> MessageType {
        return try await request(
            OperationInfo(service: "MessageTypes", operation: "UpdateMessageType", resourceType: "message_type", isMutation: true, projectId: projectId, resourceId: typeId),
            method: "PUT",
            path: "/buckets/\(projectId)/categories/\(typeId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateMessageType")
        )
    }
}
