// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListCardOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class CardsService: BaseService, @unchecked Sendable {
    public func create(columnId: Int, req: CreateCardRequest) async throws -> Card {
        return try await request(
            OperationInfo(service: "Cards", operation: "CreateCard", resourceType: "card", isMutation: true, resourceId: columnId),
            method: "POST",
            path: "/card_tables/lists/\(columnId)/cards.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateCard")
        )
    }

    public func get(cardId: Int) async throws -> Card {
        return try await request(
            OperationInfo(service: "Cards", operation: "GetCard", resourceType: "card", isMutation: false, resourceId: cardId),
            method: "GET",
            path: "/card_tables/cards/\(cardId)",
            retryConfig: Metadata.retryConfig(for: "GetCard")
        )
    }

    public func list(columnId: Int, options: ListCardOptions? = nil) async throws -> ListResult<Card> {
        return try await requestPaginated(
            OperationInfo(service: "Cards", operation: "ListCards", resourceType: "card", isMutation: false, resourceId: columnId),
            path: "/card_tables/lists/\(columnId)/cards.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListCards")
        )
    }

    public func move(cardId: Int, req: MoveCardRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Cards", operation: "MoveCard", resourceType: "card", isMutation: true, resourceId: cardId),
            method: "POST",
            path: "/card_tables/cards/\(cardId)/moves.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "MoveCard")
        )
    }

    public func update(cardId: Int, req: UpdateCardRequest) async throws -> Card {
        return try await request(
            OperationInfo(service: "Cards", operation: "UpdateCard", resourceType: "card", isMutation: true, resourceId: cardId),
            method: "PUT",
            path: "/card_tables/cards/\(cardId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateCard")
        )
    }
}
