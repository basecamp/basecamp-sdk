// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListTodolistGroupOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class TodolistGroupsService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, todolistId: Int, req: CreateTodolistGroupRequest) async throws -> TodolistGroup {
        return try await request(
            OperationInfo(service: "TodolistGroups", operation: "CreateTodolistGroup", resourceType: "todolist_group", isMutation: true, projectId: projectId, resourceId: todolistId),
            method: "POST",
            path: "/buckets/\(projectId)/todolists/\(todolistId)/groups.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTodolistGroup")
        )
    }

    public func list(projectId: Int, todolistId: Int, options: ListTodolistGroupOptions? = nil) async throws -> ListResult<TodolistGroup> {
        return try await requestPaginated(
            OperationInfo(service: "TodolistGroups", operation: "ListTodolistGroups", resourceType: "todolist_group", isMutation: false, projectId: projectId, resourceId: todolistId),
            path: "/buckets/\(projectId)/todolists/\(todolistId)/groups.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListTodolistGroups")
        )
    }

    public func reposition(projectId: Int, groupId: Int, req: RepositionTodolistGroupRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "TodolistGroups", operation: "RepositionTodolistGroup", resourceType: "todolist_group", isMutation: true, projectId: projectId, resourceId: groupId),
            method: "PUT",
            path: "/buckets/\(projectId)/todolists/\(groupId)/position.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "RepositionTodolistGroup")
        )
    }
}
