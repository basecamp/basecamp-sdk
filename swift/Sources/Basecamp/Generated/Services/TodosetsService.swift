// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class TodosetsService: BaseService, @unchecked Sendable {
    public func get(projectId: Int, todosetId: Int) async throws -> Todoset {
        return try await request(
            OperationInfo(service: "Todosets", operation: "GetTodoset", resourceType: "todoset", isMutation: false, projectId: projectId, resourceId: todosetId),
            method: "GET",
            path: "/buckets/\(projectId)/todosets/\(todosetId)",
            retryConfig: Metadata.retryConfig(for: "GetTodoset")
        )
    }
}
