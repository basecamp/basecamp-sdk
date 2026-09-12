// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListTemplateOptions: Sendable {
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

    public func createLibraryCardTable(req: CreateTemplateLibraryCardTableRequest) async throws -> CardTable {
        return try await request(
            OperationInfo(service: "Templates", operation: "CreateTemplateLibraryCardTable", resourceType: "template_library_card_table", isMutation: true),
            method: "POST",
            path: "/template_library/card_tables.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTemplateLibraryCardTable")
        )
    }

    public func createLibraryCopy(req: CreateTemplateLibraryCopyRequest) async throws -> TemplateLibraryCopy {
        return try await request(
            OperationInfo(service: "Templates", operation: "CreateTemplateLibraryCopy", resourceType: "template_library_copy", isMutation: true),
            method: "POST",
            path: "/template_library/copies.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTemplateLibraryCopy")
        )
    }

    public func createLibraryTodolist(req: CreateTemplateLibraryTodolistRequest) async throws -> Todolist {
        return try await request(
            OperationInfo(service: "Templates", operation: "CreateTemplateLibraryTodolist", resourceType: "template_library_todolist", isMutation: true),
            method: "POST",
            path: "/template_library/todolists.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTemplateLibraryTodolist")
        )
    }

    public func createTemplatification(bucketId: Int, recordingId: Int, req: CreateTemplatificationRequest) async throws -> Templatification {
        return try await request(
            OperationInfo(service: "Templates", operation: "CreateTemplatification", resourceType: "templatification", isMutation: true, projectId: bucketId, resourceId: recordingId),
            method: "POST",
            path: "/buckets/\(bucketId)/recordings/\(recordingId)/templatifications.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTemplatification")
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

    public func getLibraryCardTables() async throws -> TemplateLibraryCardTables {
        return try await request(
            OperationInfo(service: "Templates", operation: "GetTemplateLibraryCardTables", resourceType: "template_library_card_table", isMutation: false),
            method: "GET",
            path: "/template_library/card_tables.json",
            retryConfig: Metadata.retryConfig(for: "GetTemplateLibraryCardTables")
        )
    }

    public func getLibraryCopy(copyId: Int) async throws -> TemplateLibraryCopy {
        return try await request(
            OperationInfo(service: "Templates", operation: "GetTemplateLibraryCopy", resourceType: "template_library_copy", isMutation: false, resourceId: copyId),
            method: "GET",
            path: "/template_library/copies/\(copyId)",
            retryConfig: Metadata.retryConfig(for: "GetTemplateLibraryCopy")
        )
    }

    public func getLibraryTodolists() async throws -> TemplateLibraryTodolists {
        return try await request(
            OperationInfo(service: "Templates", operation: "GetTemplateLibraryTodolists", resourceType: "template_library_todolist", isMutation: false),
            method: "GET",
            path: "/template_library/todolists.json",
            retryConfig: Metadata.retryConfig(for: "GetTemplateLibraryTodolists")
        )
    }

    public func getTemplatification(bucketId: Int, recordingId: Int, templatificationId: Int) async throws -> Templatification {
        return try await request(
            OperationInfo(service: "Templates", operation: "GetTemplatification", resourceType: "templatification", isMutation: false, projectId: bucketId, resourceId: templatificationId),
            method: "GET",
            path: "/buckets/\(bucketId)/recordings/\(recordingId)/templatifications/\(templatificationId)",
            retryConfig: Metadata.retryConfig(for: "GetTemplatification")
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
