// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListCardOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class CardsService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, columnId: Int, req: CreateCardRequest) async throws -> Card {
        return try await request(
            OperationInfo(service: "Cards", operation: "CreateCard", resourceType: "card", isMutation: true, projectId: projectId, resourceId: columnId),
            method: "POST",
            path: "/buckets/\(projectId)/card_tables/lists/\(columnId)/cards.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateCard")
        )
    }

    public func get(projectId: Int, cardId: Int) async throws -> Card {
        return try await request(
            OperationInfo(service: "Cards", operation: "GetCard", resourceType: "card", isMutation: false, projectId: projectId, resourceId: cardId),
            method: "GET",
            path: "/buckets/\(projectId)/card_tables/cards/\(cardId)",
            retryConfig: Metadata.retryConfig(for: "GetCard")
        )
    }

    public func list(projectId: Int, columnId: Int, options: ListCardOptions? = nil) async throws -> ListResult<Card> {
        return try await requestPaginated(
            OperationInfo(service: "Cards", operation: "ListCards", resourceType: "card", isMutation: false, projectId: projectId, resourceId: columnId),
            path: "/buckets/\(projectId)/card_tables/lists/\(columnId)/cards.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListCards")
        )
    }

    public func move(projectId: Int, cardId: Int, req: MoveCardRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Cards", operation: "MoveCard", resourceType: "card", isMutation: true, projectId: projectId, resourceId: cardId),
            method: "POST",
            path: "/buckets/\(projectId)/card_tables/cards/\(cardId)/moves.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "MoveCard")
        )
    }

    public func update(projectId: Int, cardId: Int, req: UpdateCardRequest) async throws -> Card {
        return try await request(
            OperationInfo(service: "Cards", operation: "UpdateCard", resourceType: "card", isMutation: true, projectId: projectId, resourceId: cardId),
            method: "PUT",
            path: "/buckets/\(projectId)/card_tables/cards/\(cardId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateCard")
        )
    }
}
