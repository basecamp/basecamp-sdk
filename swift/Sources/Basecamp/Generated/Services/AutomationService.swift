// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class AutomationService: BaseService, @unchecked Sendable {
    public func listLineupMarkers() async throws -> [LineupMarker] {
        return try await request(
            OperationInfo(service: "Automation", operation: "ListLineupMarkers", resourceType: "lineup_marker", isMutation: false),
            method: "GET",
            path: "/lineup/markers.json",
            retryConfig: Metadata.retryConfig(for: "ListLineupMarkers")
        )
    }
}
