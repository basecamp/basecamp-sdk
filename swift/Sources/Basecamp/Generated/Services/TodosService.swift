// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListTodoOptions: Sendable {
    public var status: String?
    public var completed: Bool?
    public var maxItems: Int?

    public init(status: String? = nil, completed: Bool? = nil, maxItems: Int? = nil) {
        self.status = status
        self.completed = completed
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
        return try await requestPaginated(
            OperationInfo(service: "Todos", operation: "ListTodos", resourceType: "todo", isMutation: false, resourceId: todolistId),
            path: "/todolists/\(todolistId)/todos.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListTodos")
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

    public func trash(todoId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Todos", operation: "TrashTodo", resourceType: "todo", isMutation: true, resourceId: todoId),
            method: "DELETE",
            path: "/todos/\(todoId)",
            retryConfig: Metadata.retryConfig(for: "TrashTodo")
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

    public func update(todoId: Int, req: UpdateTodoRequest) async throws -> Todo {
        return try await request(
            OperationInfo(service: "Todos", operation: "UpdateTodo", resourceType: "todo", isMutation: true, resourceId: todoId),
            method: "PUT",
            path: "/todos/\(todoId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateTodo")
        )
    }
}
