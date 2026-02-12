// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListProjectOptions: Sendable {
    public var status: String?
    public var maxItems: Int?

    public init(status: String? = nil, maxItems: Int? = nil) {
        self.status = status
        self.maxItems = maxItems
    }
}


public final class ProjectsService: BaseService, @unchecked Sendable {
    public func create(req: CreateProjectRequest) async throws -> Project {
        return try await request(
            OperationInfo(service: "Projects", operation: "CreateProject", resourceType: "project", isMutation: true),
            method: "POST",
            path: "/projects.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateProject")
        )
    }

    public func get(projectId: Int) async throws -> Project {
        return try await request(
            OperationInfo(service: "Projects", operation: "GetProject", resourceType: "project", isMutation: false, projectId: projectId),
            method: "GET",
            path: "/projects/\(projectId)",
            retryConfig: Metadata.retryConfig(for: "GetProject")
        )
    }

    public func list(options: ListProjectOptions? = nil) async throws -> ListResult<Project> {
        var queryItems: [URLQueryItem] = []
        if let status = options?.status {
            queryItems.append(URLQueryItem(name: "status", value: status))
        }
        return try await requestPaginated(
            OperationInfo(service: "Projects", operation: "ListProjects", resourceType: "project", isMutation: false),
            path: "/projects.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListProjects")
        )
    }

    public func trash(projectId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Projects", operation: "TrashProject", resourceType: "project", isMutation: true, projectId: projectId),
            method: "DELETE",
            path: "/projects/\(projectId)",
            retryConfig: Metadata.retryConfig(for: "TrashProject")
        )
    }

    public func update(projectId: Int, req: UpdateProjectRequest) async throws -> Project {
        return try await request(
            OperationInfo(service: "Projects", operation: "UpdateProject", resourceType: "project", isMutation: true, projectId: projectId),
            method: "PUT",
            path: "/projects/\(projectId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateProject")
        )
    }
}
