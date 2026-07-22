// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListMessageTypeOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class MessageTypesService: BaseService, @unchecked Sendable {
    public func create(bucketId: Int, req: CreateMessageTypeRequest) async throws -> MessageType {
        return try await request(
            OperationInfo(service: "MessageTypes", operation: "CreateMessageType", resourceType: "message_type", isMutation: true, projectId: bucketId),
            method: "POST",
            path: "/buckets/\(bucketId)/categories.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateMessageType")
        )
    }

    public func delete(bucketId: Int, typeId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "MessageTypes", operation: "DeleteMessageType", resourceType: "message_type", isMutation: true, projectId: bucketId, resourceId: typeId),
            method: "DELETE",
            path: "/buckets/\(bucketId)/categories/\(typeId)",
            retryConfig: Metadata.retryConfig(for: "DeleteMessageType")
        )
    }

    public func get(bucketId: Int, typeId: Int) async throws -> MessageType {
        return try await request(
            OperationInfo(service: "MessageTypes", operation: "GetMessageType", resourceType: "message_type", isMutation: false, projectId: bucketId, resourceId: typeId),
            method: "GET",
            path: "/buckets/\(bucketId)/categories/\(typeId)",
            retryConfig: Metadata.retryConfig(for: "GetMessageType")
        )
    }

    public func list(bucketId: Int, options: ListMessageTypeOptions? = nil) async throws -> ListResult<MessageType> {
        return try await requestPaginated(
            OperationInfo(service: "MessageTypes", operation: "ListMessageTypes", resourceType: "message_type", isMutation: false, projectId: bucketId),
            path: "/buckets/\(bucketId)/categories.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListMessageTypes")
        )
    }

    public func update(bucketId: Int, typeId: Int, req: UpdateMessageTypeRequest) async throws -> MessageType {
        return try await request(
            OperationInfo(service: "MessageTypes", operation: "UpdateMessageType", resourceType: "message_type", isMutation: true, projectId: bucketId, resourceId: typeId),
            method: "PUT",
            path: "/buckets/\(bucketId)/categories/\(typeId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateMessageType")
        )
    }
}
