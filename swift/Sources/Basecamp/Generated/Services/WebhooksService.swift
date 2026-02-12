// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListWebhookOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class WebhooksService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, req: CreateWebhookRequest) async throws -> Webhook {
        return try await request(
            OperationInfo(service: "Webhooks", operation: "CreateWebhook", resourceType: "webhook", isMutation: true, projectId: projectId),
            method: "POST",
            path: "/buckets/\(projectId)/webhooks.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateWebhook")
        )
    }

    public func delete(projectId: Int, webhookId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Webhooks", operation: "DeleteWebhook", resourceType: "webhook", isMutation: true, projectId: projectId, resourceId: webhookId),
            method: "DELETE",
            path: "/buckets/\(projectId)/webhooks/\(webhookId)",
            retryConfig: Metadata.retryConfig(for: "DeleteWebhook")
        )
    }

    public func get(projectId: Int, webhookId: Int) async throws -> Webhook {
        return try await request(
            OperationInfo(service: "Webhooks", operation: "GetWebhook", resourceType: "webhook", isMutation: false, projectId: projectId, resourceId: webhookId),
            method: "GET",
            path: "/buckets/\(projectId)/webhooks/\(webhookId)",
            retryConfig: Metadata.retryConfig(for: "GetWebhook")
        )
    }

    public func list(projectId: Int, options: ListWebhookOptions? = nil) async throws -> ListResult<Webhook> {
        return try await requestPaginated(
            OperationInfo(service: "Webhooks", operation: "ListWebhooks", resourceType: "webhook", isMutation: false, projectId: projectId),
            path: "/buckets/\(projectId)/webhooks.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListWebhooks")
        )
    }

    public func update(projectId: Int, webhookId: Int, req: UpdateWebhookRequest) async throws -> Webhook {
        return try await request(
            OperationInfo(service: "Webhooks", operation: "UpdateWebhook", resourceType: "webhook", isMutation: true, projectId: projectId, resourceId: webhookId),
            method: "PUT",
            path: "/buckets/\(projectId)/webhooks/\(webhookId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateWebhook")
        )
    }
}
