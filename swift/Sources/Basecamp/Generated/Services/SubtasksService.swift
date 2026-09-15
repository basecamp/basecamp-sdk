// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListSubtaskOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class SubtasksService: BaseService, @unchecked Sendable {
    public func complete(subtaskId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Subtasks", operation: "CompleteSubtask", resourceType: "subtask", isMutation: true, resourceId: subtaskId),
            method: "POST",
            path: "/subtasks/\(subtaskId)/completion.json",
            retryConfig: Metadata.retryConfig(for: "CompleteSubtask")
        )
    }

    public func create(recordingId: Int, req: CreateSubtaskRequest) async throws -> CardStep {
        return try await request(
            OperationInfo(service: "Subtasks", operation: "CreateSubtask", resourceType: "subtask", isMutation: true, resourceId: recordingId),
            method: "POST",
            path: "/recordings/\(recordingId)/subtasks.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateSubtask")
        )
    }

    public func delete(subtaskId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Subtasks", operation: "DeleteSubtask", resourceType: "subtask", isMutation: true, resourceId: subtaskId),
            method: "DELETE",
            path: "/subtasks/\(subtaskId)",
            retryConfig: Metadata.retryConfig(for: "DeleteSubtask")
        )
    }

    public func get(subtaskId: Int) async throws -> CardStep {
        return try await request(
            OperationInfo(service: "Subtasks", operation: "GetSubtask", resourceType: "subtask", isMutation: false, resourceId: subtaskId),
            method: "GET",
            path: "/subtasks/\(subtaskId)",
            retryConfig: Metadata.retryConfig(for: "GetSubtask")
        )
    }

    public func list(recordingId: Int, options: ListSubtaskOptions? = nil) async throws -> ListResult<CardStep> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Subtasks", operation: "ListSubtasks", resourceType: "subtask", isMutation: false, resourceId: recordingId),
            path: "/recordings/\(recordingId)/subtasks.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListSubtasks")
        )
    }

    public func reposition(subtaskId: Int, req: RepositionSubtaskRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Subtasks", operation: "RepositionSubtask", resourceType: "subtask", isMutation: true, resourceId: subtaskId),
            method: "PUT",
            path: "/subtasks/\(subtaskId)/position.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "RepositionSubtask")
        )
    }

    public func uncomplete(subtaskId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Subtasks", operation: "UncompleteSubtask", resourceType: "subtask", isMutation: true, resourceId: subtaskId),
            method: "DELETE",
            path: "/subtasks/\(subtaskId)/completion.json",
            retryConfig: Metadata.retryConfig(for: "UncompleteSubtask")
        )
    }

    public func update(subtaskId: Int, req: UpdateSubtaskRequest) async throws -> CardStep {
        return try await request(
            OperationInfo(service: "Subtasks", operation: "UpdateSubtask", resourceType: "subtask", isMutation: true, resourceId: subtaskId),
            method: "PUT",
            path: "/subtasks/\(subtaskId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateSubtask")
        )
    }
}
