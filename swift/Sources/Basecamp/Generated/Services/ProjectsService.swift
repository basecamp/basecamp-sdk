// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListProjectOptions: Sendable {
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

public struct UpdateProjectWallpaperProjectOptions: Sendable {
    /// How the wallpaper is laid out. Omitted, the project's current mode is kept.
    public var mode: String?

    public init(mode: String? = nil) {
        self.mode = mode
    }
}


public final class ProjectsService: BaseService, @unchecked Sendable {
    public func archive(projectId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Projects", operation: "ArchiveProject", resourceType: "project", isMutation: true, projectId: projectId),
            method: "PUT",
            path: "/projects/\(projectId)/status/archived.json",
            retryConfig: Metadata.retryConfig(for: "ArchiveProject")
        )
    }

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
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Projects", operation: "ListProjects", resourceType: "project", isMutation: false),
            path: "/projects.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListProjects")
        )
    }

    public func listRecentProjects() async throws -> [Project] {
        return try await request(
            OperationInfo(service: "Projects", operation: "ListRecentProjects", resourceType: "project", isMutation: false),
            method: "GET",
            path: "/my/recent_projects.json",
            retryConfig: Metadata.retryConfig(for: "ListRecentProjects")
        )
    }

    public func recordProjectVisit(projectId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Projects", operation: "RecordProjectVisit", resourceType: "project", isMutation: true, projectId: projectId),
            method: "POST",
            path: "/projects/\(projectId)/recent_visit.json",
            retryConfig: Metadata.retryConfig(for: "RecordProjectVisit")
        )
    }

    public func removeProjectWallpaper(projectId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Projects", operation: "RemoveProjectWallpaper", resourceType: "project_wallpaper", isMutation: true, projectId: projectId),
            method: "DELETE",
            path: "/projects/\(projectId)/wallpaper.json",
            retryConfig: Metadata.retryConfig(for: "RemoveProjectWallpaper")
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

    public func unarchive(projectId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Projects", operation: "UnarchiveProject", resourceType: "project", isMutation: true, projectId: projectId),
            method: "PUT",
            path: "/projects/\(projectId)/status/active.json",
            retryConfig: Metadata.retryConfig(for: "UnarchiveProject")
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

    public func updateProjectWallpaper(projectId: Int, data: Data, filename: String, contentType: String, options: UpdateProjectWallpaperProjectOptions? = nil) async throws {
        var queryItems: [URLQueryItem] = []
        if let mode = options?.mode {
            queryItems.append(URLQueryItem(name: "mode", value: mode))
        }
        let boundary = UUID().uuidString
        let multipartContentType = "multipart/form-data; boundary=\(boundary)"
        // Sanitize user-provided values to prevent CRLF injection / quote breakout
        let safeFilename = filename.replacingOccurrences(of: "\r", with: "").replacingOccurrences(of: "\n", with: "").replacingOccurrences(of: "\\", with: "\\\\").replacingOccurrences(of: "\"", with: "\\\"")
        let safeContentType = contentType.replacingOccurrences(of: "\r", with: "").replacingOccurrences(of: "\n", with: "")
        var body = Data()
        body.append(contentsOf: "--\(boundary)\r\n".utf8)
        body.append(contentsOf: "Content-Disposition: form-data; name=\"wallpaper\"; filename=\"\(safeFilename)\"\r\n".utf8)
        body.append(contentsOf: "Content-Type: \(safeContentType)\r\n\r\n".utf8)
        body.append(data)
        body.append(contentsOf: "\r\n--\(boundary)--\r\n".utf8)
        let multipartBody = body
        try await requestVoid(
            OperationInfo(service: "Projects", operation: "UpdateProjectWallpaper", resourceType: "project_wallpaper", isMutation: true, projectId: projectId),
            method: "PUT",
            path: "/projects/\(projectId)/wallpaper.json" + queryString(queryItems),
            body: multipartBody,
            contentType: multipartContentType,
            retryConfig: Metadata.retryConfig(for: "UpdateProjectWallpaper")
        )
    }
}
