// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class CardTablesService: BaseService, @unchecked Sendable {
    public func get(cardTableId: Int) async throws -> CardTable {
        return try await request(
            OperationInfo(service: "CardTables", operation: "GetCardTable", resourceType: "card_table", isMutation: false, resourceId: cardTableId),
            method: "GET",
            path: "/card_tables/\(cardTableId)",
            retryConfig: Metadata.retryConfig(for: "GetCardTable")
        )
    }
}
