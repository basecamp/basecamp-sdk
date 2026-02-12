// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class SubscriptionsService: BaseService, @unchecked Sendable {
    public func get(projectId: Int, recordingId: Int) async throws -> Subscription {
        return try await request(
            OperationInfo(service: "Subscriptions", operation: "GetSubscription", resourceType: "subscription", isMutation: false, projectId: projectId, resourceId: recordingId),
            method: "GET",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/subscription.json",
            retryConfig: Metadata.retryConfig(for: "GetSubscription")
        )
    }

    public func subscribe(projectId: Int, recordingId: Int) async throws -> Subscription {
        return try await request(
            OperationInfo(service: "Subscriptions", operation: "Subscribe", resourceType: "resource", isMutation: true, projectId: projectId, resourceId: recordingId),
            method: "POST",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/subscription.json",
            retryConfig: Metadata.retryConfig(for: "Subscribe")
        )
    }

    public func unsubscribe(projectId: Int, recordingId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Subscriptions", operation: "Unsubscribe", resourceType: "resource", isMutation: true, projectId: projectId, resourceId: recordingId),
            method: "DELETE",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/subscription.json",
            retryConfig: Metadata.retryConfig(for: "Unsubscribe")
        )
    }

    public func update(projectId: Int, recordingId: Int, req: UpdateSubscriptionRequest) async throws -> Subscription {
        return try await request(
            OperationInfo(service: "Subscriptions", operation: "UpdateSubscription", resourceType: "subscription", isMutation: true, projectId: projectId, resourceId: recordingId),
            method: "PUT",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/subscription.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateSubscription")
        )
    }
}
