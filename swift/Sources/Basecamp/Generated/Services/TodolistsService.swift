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
    public func create(todosetId: Int, req: CreateTodolistRequest) async throws -> Todolist {
        return try await request(
            OperationInfo(service: "Todolists", operation: "CreateTodolist", resourceType: "todolist", isMutation: true, resourceId: todosetId),
            method: "POST",
            path: "/todosets/\(todosetId)/todolists.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTodolist")
        )
    }

    public func get(id: Int) async throws -> TodolistOrGroup {
        return try await request(
            OperationInfo(service: "Todolists", operation: "GetTodolistOrGroup", resourceType: "todolist_or_group", isMutation: false),
            method: "GET",
            path: "/todolists/\(id)",
            retryConfig: Metadata.retryConfig(for: "GetTodolistOrGroup")
        )
    }

    public func list(todosetId: Int, options: ListTodolistOptions? = nil) async throws -> ListResult<Todolist> {
        var queryItems: [URLQueryItem] = []
        if let status = options?.status {
            queryItems.append(URLQueryItem(name: "status", value: status))
        }
        return try await requestPaginated(
            OperationInfo(service: "Todolists", operation: "ListTodolists", resourceType: "todolist", isMutation: false, resourceId: todosetId),
            path: "/todosets/\(todosetId)/todolists.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListTodolists")
        )
    }

    public func update(id: Int, req: UpdateTodolistOrGroupRequest) async throws -> TodolistOrGroup {
        return try await request(
            OperationInfo(service: "Todolists", operation: "UpdateTodolistOrGroup", resourceType: "todolist_or_group", isMutation: true),
            method: "PUT",
            path: "/todolists/\(id)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateTodolistOrGroup")
        )
    }
}
