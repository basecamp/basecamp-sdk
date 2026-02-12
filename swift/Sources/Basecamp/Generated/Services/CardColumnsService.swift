// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class CardColumnsService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, cardTableId: Int, req: CreateCardColumnRequest) async throws -> CardColumn {
        return try await request(
            OperationInfo(service: "CardColumns", operation: "CreateCardColumn", resourceType: "card_column", isMutation: true, projectId: projectId, resourceId: cardTableId),
            method: "POST",
            path: "/buckets/\(projectId)/card_tables/\(cardTableId)/columns.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateCardColumn")
        )
    }

    public func disableOnHold(projectId: Int, columnId: Int) async throws -> CardColumn {
        return try await request(
            OperationInfo(service: "CardColumns", operation: "DisableCardColumnOnHold", resourceType: "card_column_on_hold", isMutation: true, projectId: projectId, resourceId: columnId),
            method: "DELETE",
            path: "/buckets/\(projectId)/card_tables/columns/\(columnId)/on_hold.json",
            retryConfig: Metadata.retryConfig(for: "DisableCardColumnOnHold")
        )
    }

    public func enableOnHold(projectId: Int, columnId: Int) async throws -> CardColumn {
        return try await request(
            OperationInfo(service: "CardColumns", operation: "EnableCardColumnOnHold", resourceType: "card_column_on_hold", isMutation: true, projectId: projectId, resourceId: columnId),
            method: "POST",
            path: "/buckets/\(projectId)/card_tables/columns/\(columnId)/on_hold.json",
            retryConfig: Metadata.retryConfig(for: "EnableCardColumnOnHold")
        )
    }

    public func get(projectId: Int, columnId: Int) async throws -> CardColumn {
        return try await request(
            OperationInfo(service: "CardColumns", operation: "GetCardColumn", resourceType: "card_column", isMutation: false, projectId: projectId, resourceId: columnId),
            method: "GET",
            path: "/buckets/\(projectId)/card_tables/columns/\(columnId)",
            retryConfig: Metadata.retryConfig(for: "GetCardColumn")
        )
    }

    public func move(projectId: Int, cardTableId: Int, req: MoveCardColumnRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "CardColumns", operation: "MoveCardColumn", resourceType: "card_column", isMutation: true, projectId: projectId, resourceId: cardTableId),
            method: "POST",
            path: "/buckets/\(projectId)/card_tables/\(cardTableId)/moves.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "MoveCardColumn")
        )
    }

    public func setColor(projectId: Int, columnId: Int, req: SetCardColumnColorRequest) async throws -> CardColumn {
        return try await request(
            OperationInfo(service: "CardColumns", operation: "SetCardColumnColor", resourceType: "card_column_color", isMutation: true, projectId: projectId, resourceId: columnId),
            method: "PUT",
            path: "/buckets/\(projectId)/card_tables/columns/\(columnId)/color.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "SetCardColumnColor")
        )
    }

    public func subscribeToColumn(projectId: Int, columnId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "CardColumns", operation: "SubscribeToCardColumn", resourceType: "to_card_column", isMutation: true, projectId: projectId, resourceId: columnId),
            method: "POST",
            path: "/buckets/\(projectId)/card_tables/lists/\(columnId)/subscription.json",
            retryConfig: Metadata.retryConfig(for: "SubscribeToCardColumn")
        )
    }

    public func unsubscribeFromColumn(projectId: Int, columnId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "CardColumns", operation: "UnsubscribeFromCardColumn", resourceType: "from_card_column", isMutation: true, projectId: projectId, resourceId: columnId),
            method: "DELETE",
            path: "/buckets/\(projectId)/card_tables/lists/\(columnId)/subscription.json",
            retryConfig: Metadata.retryConfig(for: "UnsubscribeFromCardColumn")
        )
    }

    public func update(projectId: Int, columnId: Int, req: UpdateCardColumnRequest) async throws -> CardColumn {
        return try await request(
            OperationInfo(service: "CardColumns", operation: "UpdateCardColumn", resourceType: "card_column", isMutation: true, projectId: projectId, resourceId: columnId),
            method: "PUT",
            path: "/buckets/\(projectId)/card_tables/columns/\(columnId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateCardColumn")
        )
    }
}
