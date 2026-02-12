// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class LineupService: BaseService, @unchecked Sendable {
    public func create(req: CreateLineupMarkerRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Lineup", operation: "CreateLineupMarker", resourceType: "lineup_marker", isMutation: true),
            method: "POST",
            path: "/lineup/markers.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateLineupMarker")
        )
    }

    public func delete(markerId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Lineup", operation: "DeleteLineupMarker", resourceType: "lineup_marker", isMutation: true, resourceId: markerId),
            method: "DELETE",
            path: "/lineup/markers/\(markerId)",
            retryConfig: Metadata.retryConfig(for: "DeleteLineupMarker")
        )
    }

    public func update(markerId: Int, req: UpdateLineupMarkerRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Lineup", operation: "UpdateLineupMarker", resourceType: "lineup_marker", isMutation: true, resourceId: markerId),
            method: "PUT",
            path: "/lineup/markers/\(markerId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateLineupMarker")
        )
    }
}
