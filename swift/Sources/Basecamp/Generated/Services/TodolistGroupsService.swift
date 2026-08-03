// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListTodolistGroupOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class TodolistGroupsService: BaseService, @unchecked Sendable {
    public func create(todolistId: Int, req: CreateTodolistGroupRequest) async throws -> TodolistGroup {
        return try await request(
            OperationInfo(service: "TodolistGroups", operation: "CreateTodolistGroup", resourceType: "todolist_group", isMutation: true, resourceId: todolistId),
            method: "POST",
            path: "/todolists/\(todolistId)/groups.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTodolistGroup")
        )
    }

    public func list(todolistId: Int, options: ListTodolistGroupOptions? = nil) async throws -> ListResult<TodolistGroup> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "TodolistGroups", operation: "ListTodolistGroups", resourceType: "todolist_group", isMutation: false, resourceId: todolistId),
            path: "/todolists/\(todolistId)/groups.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListTodolistGroups")
        )
    }

    public func reposition(groupId: Int, req: RepositionTodolistGroupRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "TodolistGroups", operation: "RepositionTodolistGroup", resourceType: "todolist_group", isMutation: true, resourceId: groupId),
            method: "PUT",
            path: "/todolists/groups/\(groupId)/position.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "RepositionTodolistGroup")
        )
    }
}
