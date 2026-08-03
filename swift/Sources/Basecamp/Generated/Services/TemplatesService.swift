// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListTemplateOptions: Sendable {
    /// active|archived|trashed
    public var status: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(status: String? = nil, page: Int? = nil, maxItems: Int? = nil) {
        self.status = status
        self.page = page
        self.maxItems = maxItems
    }
}


public final class TemplatesService: BaseService, @unchecked Sendable {
    public func createProject(templateId: Int, req: CreateProjectFromTemplateRequest) async throws -> ProjectConstruction {
        return try await request(
            OperationInfo(service: "Templates", operation: "CreateProjectFromTemplate", resourceType: "project_from_template", isMutation: true, resourceId: templateId),
            method: "POST",
            path: "/templates/\(templateId)/project_constructions.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateProjectFromTemplate")
        )
    }

    public func create(req: CreateTemplateRequest) async throws -> Template {
        return try await request(
            OperationInfo(service: "Templates", operation: "CreateTemplate", resourceType: "template", isMutation: true),
            method: "POST",
            path: "/templates.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTemplate")
        )
    }

    public func delete(templateId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Templates", operation: "DeleteTemplate", resourceType: "template", isMutation: true, resourceId: templateId),
            method: "DELETE",
            path: "/templates/\(templateId)",
            retryConfig: Metadata.retryConfig(for: "DeleteTemplate")
        )
    }

    public func getConstruction(templateId: Int, constructionId: Int) async throws -> ProjectConstruction {
        return try await request(
            OperationInfo(service: "Templates", operation: "GetProjectConstruction", resourceType: "project_construction", isMutation: false, resourceId: constructionId),
            method: "GET",
            path: "/templates/\(templateId)/project_constructions/\(constructionId)",
            retryConfig: Metadata.retryConfig(for: "GetProjectConstruction")
        )
    }

    public func get(templateId: Int) async throws -> Template {
        return try await request(
            OperationInfo(service: "Templates", operation: "GetTemplate", resourceType: "template", isMutation: false, resourceId: templateId),
            method: "GET",
            path: "/templates/\(templateId)",
            retryConfig: Metadata.retryConfig(for: "GetTemplate")
        )
    }

    public func list(options: ListTemplateOptions? = nil) async throws -> ListResult<Template> {
        var queryItems: [URLQueryItem] = []
        if let status = options?.status {
            queryItems.append(URLQueryItem(name: "status", value: status))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Templates", operation: "ListTemplates", resourceType: "template", isMutation: false),
            path: "/templates.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListTemplates")
        )
    }

    public func update(templateId: Int, req: UpdateTemplateRequest) async throws -> Template {
        return try await request(
            OperationInfo(service: "Templates", operation: "UpdateTemplate", resourceType: "template", isMutation: true, resourceId: templateId),
            method: "PUT",
            path: "/templates/\(templateId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateTemplate")
        )
    }
}
