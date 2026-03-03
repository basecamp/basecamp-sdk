// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class TodosetsService: BaseService, @unchecked Sendable {
    public func get(todosetId: Int) async throws -> Todoset {
        return try await request(
            OperationInfo(service: "Todosets", operation: "GetTodoset", resourceType: "todoset", isMutation: false, resourceId: todosetId),
            method: "GET",
            path: "/todosets/\(todosetId)",
            retryConfig: Metadata.retryConfig(for: "GetTodoset")
        )
    }
}
