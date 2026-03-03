// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class ClientVisibilityService: BaseService, @unchecked Sendable {
    public func setVisibility(recordingId: Int, req: SetClientVisibilityRequest) async throws -> Recording {
        return try await request(
            OperationInfo(service: "ClientVisibility", operation: "SetClientVisibility", resourceType: "client_visibility", isMutation: true, resourceId: recordingId),
            method: "PUT",
            path: "/recordings/\(recordingId)/client_visibility.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "SetClientVisibility")
        )
    }
}
