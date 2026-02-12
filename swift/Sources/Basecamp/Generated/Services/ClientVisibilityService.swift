// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class ClientVisibilityService: BaseService, @unchecked Sendable {
    public func setVisibility(projectId: Int, recordingId: Int, req: SetClientVisibilityRequest) async throws -> Recording {
        return try await request(
            OperationInfo(service: "ClientVisibility", operation: "SetClientVisibility", resourceType: "client_visibility", isMutation: true, projectId: projectId, resourceId: recordingId),
            method: "PUT",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/client_visibility.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "SetClientVisibility")
        )
    }
}
