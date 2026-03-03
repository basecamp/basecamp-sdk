// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListWebhookOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class WebhooksService: BaseService, @unchecked Sendable {
    public func create(bucketId: Int, req: CreateWebhookRequest) async throws -> Webhook {
        return try await request(
            OperationInfo(service: "Webhooks", operation: "CreateWebhook", resourceType: "webhook", isMutation: true, resourceId: bucketId),
            method: "POST",
            path: "/buckets/\(bucketId)/webhooks.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateWebhook")
        )
    }

    public func delete(webhookId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Webhooks", operation: "DeleteWebhook", resourceType: "webhook", isMutation: true, resourceId: webhookId),
            method: "DELETE",
            path: "/webhooks/\(webhookId)",
            retryConfig: Metadata.retryConfig(for: "DeleteWebhook")
        )
    }

    public func get(webhookId: Int) async throws -> Webhook {
        return try await request(
            OperationInfo(service: "Webhooks", operation: "GetWebhook", resourceType: "webhook", isMutation: false, resourceId: webhookId),
            method: "GET",
            path: "/webhooks/\(webhookId)",
            retryConfig: Metadata.retryConfig(for: "GetWebhook")
        )
    }

    public func list(bucketId: Int, options: ListWebhookOptions? = nil) async throws -> ListResult<Webhook> {
        return try await requestPaginated(
            OperationInfo(service: "Webhooks", operation: "ListWebhooks", resourceType: "webhook", isMutation: false, resourceId: bucketId),
            path: "/buckets/\(bucketId)/webhooks.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListWebhooks")
        )
    }

    public func update(webhookId: Int, req: UpdateWebhookRequest) async throws -> Webhook {
        return try await request(
            OperationInfo(service: "Webhooks", operation: "UpdateWebhook", resourceType: "webhook", isMutation: true, resourceId: webhookId),
            method: "PUT",
            path: "/webhooks/\(webhookId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateWebhook")
        )
    }
}
