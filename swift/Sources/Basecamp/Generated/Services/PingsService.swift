// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class PingsService: BaseService, @unchecked Sendable {
    public func create(req: CreatePingRequest) async throws -> Ping {
        return try await request(
            OperationInfo(service: "Pings", operation: "CreatePing", resourceType: "ping", isMutation: true),
            method: "POST",
            path: "/circles.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreatePing")
        )
    }

    public func get(pingId: Int) async throws -> Ping {
        return try await request(
            OperationInfo(service: "Pings", operation: "GetPing", resourceType: "ping", isMutation: false, resourceId: pingId),
            method: "GET",
            path: "/circles/\(pingId)",
            retryConfig: Metadata.retryConfig(for: "GetPing")
        )
    }

    public func list() async throws -> [Ping] {
        return try await request(
            OperationInfo(service: "Pings", operation: "ListPings", resourceType: "ping", isMutation: false),
            method: "GET",
            path: "/circles.json",
            retryConfig: Metadata.retryConfig(for: "ListPings")
        )
    }
}
