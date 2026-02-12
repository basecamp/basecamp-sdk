// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListTodolistOptions: Sendable {
    public var status: String?
    public var maxItems: Int?

    public init(status: String? = nil, maxItems: Int? = nil) {
        self.status = status
        self.maxItems = maxItems
    }
}


public final class TodolistsService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, todosetId: Int, req: CreateTodolistRequest) async throws -> Todolist {
        return try await request(
            OperationInfo(service: "Todolists", operation: "CreateTodolist", resourceType: "todolist", isMutation: true, projectId: projectId, resourceId: todosetId),
            method: "POST",
            path: "/buckets/\(projectId)/todosets/\(todosetId)/todolists.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTodolist")
        )
    }

    public func get(projectId: Int, id: Int) async throws -> TodolistOrGroup {
        return try await request(
            OperationInfo(service: "Todolists", operation: "GetTodolistOrGroup", resourceType: "todolist_or_group", isMutation: false, projectId: projectId),
            method: "GET",
            path: "/buckets/\(projectId)/todolists/\(id)",
            retryConfig: Metadata.retryConfig(for: "GetTodolistOrGroup")
        )
    }

    public func list(projectId: Int, todosetId: Int, options: ListTodolistOptions? = nil) async throws -> ListResult<Todolist> {
        var queryItems: [URLQueryItem] = []
        if let status = options?.status {
            queryItems.append(URLQueryItem(name: "status", value: status))
        }
        return try await requestPaginated(
            OperationInfo(service: "Todolists", operation: "ListTodolists", resourceType: "todolist", isMutation: false, projectId: projectId, resourceId: todosetId),
            path: "/buckets/\(projectId)/todosets/\(todosetId)/todolists.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListTodolists")
        )
    }

    public func update(projectId: Int, id: Int, req: UpdateTodolistOrGroupRequest) async throws -> TodolistOrGroup {
        return try await request(
            OperationInfo(service: "Todolists", operation: "UpdateTodolistOrGroup", resourceType: "todolist_or_group", isMutation: true, projectId: projectId),
            method: "PUT",
            path: "/buckets/\(projectId)/todolists/\(id)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateTodolistOrGroup")
        )
    }
}
