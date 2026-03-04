// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class SubscriptionsService: BaseService, @unchecked Sendable {
    public func get(recordingId: Int) async throws -> Subscription {
        return try await request(
            OperationInfo(service: "Subscriptions", operation: "GetSubscription", resourceType: "subscription", isMutation: false, resourceId: recordingId),
            method: "GET",
            path: "/recordings/\(recordingId)/subscription.json",
            retryConfig: Metadata.retryConfig(for: "GetSubscription")
        )
    }

    public func subscribe(recordingId: Int) async throws -> Subscription {
        return try await request(
            OperationInfo(service: "Subscriptions", operation: "Subscribe", resourceType: "resource", isMutation: true, resourceId: recordingId),
            method: "POST",
            path: "/recordings/\(recordingId)/subscription.json",
            retryConfig: Metadata.retryConfig(for: "Subscribe")
        )
    }

    public func unsubscribe(recordingId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Subscriptions", operation: "Unsubscribe", resourceType: "resource", isMutation: true, resourceId: recordingId),
            method: "DELETE",
            path: "/recordings/\(recordingId)/subscription.json",
            retryConfig: Metadata.retryConfig(for: "Unsubscribe")
        )
    }

    public func update(recordingId: Int, req: UpdateSubscriptionRequest) async throws -> Subscription {
        return try await request(
            OperationInfo(service: "Subscriptions", operation: "UpdateSubscription", resourceType: "subscription", isMutation: true, resourceId: recordingId),
            method: "PUT",
            path: "/recordings/\(recordingId)/subscription.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateSubscription")
        )
    }
}
