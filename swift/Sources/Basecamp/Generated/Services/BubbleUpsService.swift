// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class BubbleUpsService: BaseService, @unchecked Sendable {
    public func createBubbleUp(recordingId: Int, req: CreateBubbleUpRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "BubbleUps", operation: "CreateBubbleUp", resourceType: "bubble_up", isMutation: true, resourceId: recordingId),
            method: "POST",
            path: "/recordings/\(recordingId)/bubble_up.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateBubbleUp")
        )
    }

    public func deleteBubbleUp(recordingId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "BubbleUps", operation: "DeleteBubbleUp", resourceType: "bubble_up", isMutation: true, resourceId: recordingId),
            method: "DELETE",
            path: "/recordings/\(recordingId)/bubble_up.json",
            retryConfig: Metadata.retryConfig(for: "DeleteBubbleUp")
        )
    }
}
