// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class DoorsService: BaseService, @unchecked Sendable {
    public func create(bucketId: Int, req: CreateDoorRequest) async throws -> Door {
        return try await request(
            OperationInfo(service: "Doors", operation: "CreateDoor", resourceType: "door", isMutation: true, projectId: bucketId),
            method: "POST",
            path: "/buckets/\(bucketId)/dock/doors.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateDoor")
        )
    }

    public func delete(doorId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Doors", operation: "DeleteDoor", resourceType: "door", isMutation: true, resourceId: doorId),
            method: "DELETE",
            path: "/dock/doors/\(doorId)",
            retryConfig: Metadata.retryConfig(for: "DeleteDoor")
        )
    }

    public func get(doorId: Int) async throws -> Door {
        return try await request(
            OperationInfo(service: "Doors", operation: "GetDoor", resourceType: "door", isMutation: false, resourceId: doorId),
            method: "GET",
            path: "/dock/doors/\(doorId)",
            retryConfig: Metadata.retryConfig(for: "GetDoor")
        )
    }

    public func list(bucketId: Int) async throws -> [Door] {
        return try await request(
            OperationInfo(service: "Doors", operation: "ListDoors", resourceType: "door", isMutation: false, projectId: bucketId),
            method: "GET",
            path: "/buckets/\(bucketId)/dock/doors.json",
            retryConfig: Metadata.retryConfig(for: "ListDoors")
        )
    }

    public func update(doorId: Int, req: UpdateDoorRequest) async throws -> Door {
        return try await request(
            OperationInfo(service: "Doors", operation: "UpdateDoor", resourceType: "door", isMutation: true, resourceId: doorId),
            method: "PUT",
            path: "/dock/doors/\(doorId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateDoor")
        )
    }
}
