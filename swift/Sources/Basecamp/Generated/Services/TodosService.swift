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
    public func complete(projectId: Int, todoId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Todos", operation: "CompleteTodo", resourceType: "todo", isMutation: true, projectId: projectId, resourceId: todoId),
            method: "POST",
            path: "/buckets/\(projectId)/todos/\(todoId)/completion.json",
            retryConfig: Metadata.retryConfig(for: "CompleteTodo")
        )
    }

    public func create(projectId: Int, todolistId: Int, req: CreateTodoRequest) async throws -> Todo {
        return try await request(
            OperationInfo(service: "Todos", operation: "CreateTodo", resourceType: "todo", isMutation: true, projectId: projectId, resourceId: todolistId),
            method: "POST",
            path: "/buckets/\(projectId)/todolists/\(todolistId)/todos.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTodo")
        )
    }

    public func get(projectId: Int, todoId: Int) async throws -> Todo {
        return try await request(
            OperationInfo(service: "Todos", operation: "GetTodo", resourceType: "todo", isMutation: false, projectId: projectId, resourceId: todoId),
            method: "GET",
            path: "/buckets/\(projectId)/todos/\(todoId)",
            retryConfig: Metadata.retryConfig(for: "GetTodo")
        )
    }

    public func list(projectId: Int, todolistId: Int, options: ListTodoOptions? = nil) async throws -> ListResult<Todo> {
        var queryItems: [URLQueryItem] = []
        if let status = options?.status {
            queryItems.append(URLQueryItem(name: "status", value: status))
        }
        if let completed = options?.completed {
            queryItems.append(URLQueryItem(name: "completed", value: String(completed)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Todos", operation: "ListTodos", resourceType: "todo", isMutation: false, projectId: projectId, resourceId: todolistId),
            path: "/buckets/\(projectId)/todolists/\(todolistId)/todos.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListTodos")
        )
    }

    public func reposition(projectId: Int, todoId: Int, req: RepositionTodoRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Todos", operation: "RepositionTodo", resourceType: "todo", isMutation: true, projectId: projectId, resourceId: todoId),
            method: "PUT",
            path: "/buckets/\(projectId)/todos/\(todoId)/position.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "RepositionTodo")
        )
    }

    public func trash(projectId: Int, todoId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Todos", operation: "TrashTodo", resourceType: "todo", isMutation: true, projectId: projectId, resourceId: todoId),
            method: "DELETE",
            path: "/buckets/\(projectId)/todos/\(todoId)",
            retryConfig: Metadata.retryConfig(for: "TrashTodo")
        )
    }

    public func uncomplete(projectId: Int, todoId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Todos", operation: "UncompleteTodo", resourceType: "todo", isMutation: true, projectId: projectId, resourceId: todoId),
            method: "DELETE",
            path: "/buckets/\(projectId)/todos/\(todoId)/completion.json",
            retryConfig: Metadata.retryConfig(for: "UncompleteTodo")
        )
    }

    public func update(projectId: Int, todoId: Int, req: UpdateTodoRequest) async throws -> Todo {
        return try await request(
            OperationInfo(service: "Todos", operation: "UpdateTodo", resourceType: "todo", isMutation: true, projectId: projectId, resourceId: todoId),
            method: "PUT",
            path: "/buckets/\(projectId)/todos/\(todoId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateTodo")
        )
    }
}
