// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListTodoOptions: Sendable {
    /// active|archived|trashed
    public var status: String?
    public var completed: Bool?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        status: String? = nil,
        completed: Bool? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.status = status
        self.completed = completed
        self.page = page
        self.maxItems = maxItems
    }
}


public final class TodosService: BaseService, @unchecked Sendable {
    public func complete(todoId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Todos", operation: "CompleteTodo", resourceType: "todo", isMutation: true, resourceId: todoId),
            method: "POST",
            path: "/todos/\(todoId)/completion.json",
            retryConfig: Metadata.retryConfig(for: "CompleteTodo")
        )
    }

    public func create(todolistId: Int, req: CreateTodoRequest) async throws -> Todo {
        return try await request(
            OperationInfo(service: "Todos", operation: "CreateTodo", resourceType: "todo", isMutation: true, resourceId: todolistId),
            method: "POST",
            path: "/todolists/\(todolistId)/todos.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTodo")
        )
    }

    public func createTodosetTodo(bucketId: Int, todosetId: Int, req: CreateTodosetTodoRequest) async throws -> Todo {
        return try await request(
            OperationInfo(service: "Todos", operation: "CreateTodosetTodo", resourceType: "todo", isMutation: true, projectId: bucketId, resourceId: todosetId),
            method: "POST",
            path: "/buckets/\(bucketId)/todosets/\(todosetId)/todos.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTodosetTodo")
        )
    }

    public func get(todoId: Int) async throws -> Todo {
        return try await request(
            OperationInfo(service: "Todos", operation: "GetTodo", resourceType: "todo", isMutation: false, resourceId: todoId),
            method: "GET",
            path: "/todos/\(todoId)",
            retryConfig: Metadata.retryConfig(for: "GetTodo")
        )
    }

    public func list(todolistId: Int, options: ListTodoOptions? = nil) async throws -> ListResult<Todo> {
        var queryItems: [URLQueryItem] = []
        if let status = options?.status {
            queryItems.append(URLQueryItem(name: "status", value: status))
        }
        if let completed = options?.completed {
            queryItems.append(URLQueryItem(name: "completed", value: String(completed)))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Todos", operation: "ListTodos", resourceType: "todo", isMutation: false, resourceId: todolistId),
            path: "/todolists/\(todolistId)/todos.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListTodos")
        )
    }

    public func replace(todoId: Int, req: ReplaceTodoRequest) async throws -> Todo {
        return try await request(
            OperationInfo(service: "Todos", operation: "ReplaceTodo", resourceType: "todo", isMutation: true, resourceId: todoId),
            method: "PUT",
            path: "/todos/\(todoId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "ReplaceTodo")
        )
    }

    public func reposition(todoId: Int, req: RepositionTodoRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Todos", operation: "RepositionTodo", resourceType: "todo", isMutation: true, resourceId: todoId),
            method: "PUT",
            path: "/todos/\(todoId)/position.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "RepositionTodo")
        )
    }

    public func uncomplete(todoId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Todos", operation: "UncompleteTodo", resourceType: "todo", isMutation: true, resourceId: todoId),
            method: "DELETE",
            path: "/todos/\(todoId)/completion.json",
            retryConfig: Metadata.retryConfig(for: "UncompleteTodo")
        )
    }
}
