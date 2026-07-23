// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class WormholesService: BaseService, @unchecked Sendable {
    public func create(bucketId: Int, cardTableId: Int, req: CreateWormholeRequest) async throws -> Wormhole {
        return try await request(
            OperationInfo(service: "Wormholes", operation: "CreateWormhole", resourceType: "wormhole", isMutation: true, resourceId: cardTableId),
            method: "POST",
            path: "/buckets/\(bucketId)/card_tables/\(cardTableId)/wormholes.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateWormhole")
        )
    }

    public func delete(bucketId: Int, wormholeId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Wormholes", operation: "DeleteWormhole", resourceType: "wormhole", isMutation: true, resourceId: wormholeId),
            method: "DELETE",
            path: "/buckets/\(bucketId)/card_tables/wormholes/\(wormholeId)",
            retryConfig: Metadata.retryConfig(for: "DeleteWormhole")
        )
    }

    public func update(bucketId: Int, wormholeId: Int, req: UpdateWormholeRequest) async throws -> Wormhole {
        return try await request(
            OperationInfo(service: "Wormholes", operation: "UpdateWormhole", resourceType: "wormhole", isMutation: true, resourceId: wormholeId),
            method: "PUT",
            path: "/buckets/\(bucketId)/card_tables/wormholes/\(wormholeId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateWormhole")
        )
    }
}
