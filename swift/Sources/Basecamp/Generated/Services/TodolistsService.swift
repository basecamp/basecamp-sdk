// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListTodolistOptions: Sendable {
    /// active|archived|trashed
    public var status: String?
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(status: String? = nil, page: Int? = nil, maxItems: Int? = nil) {
        self.status = status
        self.page = page
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

    public func get(id: Int) async throws -> Todolist {
        return try await request(
            OperationInfo(service: "Todolists", operation: "GetTodolistOrGroup", resourceType: "todolist_or_group", isMutation: false, resourceId: id),
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
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Todolists", operation: "ListTodolists", resourceType: "todolist", isMutation: false, resourceId: todosetId),
            path: "/todosets/\(todosetId)/todolists.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListTodolists")
        )
    }

    public func reposition(todolistId: Int, req: RepositionTodolistRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Todolists", operation: "RepositionTodolist", resourceType: "todolist", isMutation: true, resourceId: todolistId),
            method: "PUT",
            path: "/todosets/todolists/\(todolistId)/position.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "RepositionTodolist")
        )
    }

    public func replace(id: Int, req: UpdateTodolistOrGroupRequest) async throws -> Todolist {
        return try await request(
            OperationInfo(service: "Todolists", operation: "UpdateTodolistOrGroup", resourceType: "todolist_or_group", isMutation: true, resourceId: id),
            method: "PUT",
            path: "/todolists/\(id)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateTodolistOrGroup")
        )
    }
}
