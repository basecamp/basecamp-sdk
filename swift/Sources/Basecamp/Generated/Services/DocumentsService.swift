// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListDocumentOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class DocumentsService: BaseService, @unchecked Sendable {
    public func create(vaultId: Int, req: CreateDocumentRequest) async throws -> Document {
        return try await request(
            OperationInfo(service: "Documents", operation: "CreateDocument", resourceType: "document", isMutation: true, resourceId: vaultId),
            method: "POST",
            path: "/vaults/\(vaultId)/documents.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateDocument")
        )
    }

    public func get(documentId: Int) async throws -> Document {
        return try await request(
            OperationInfo(service: "Documents", operation: "GetDocument", resourceType: "document", isMutation: false, resourceId: documentId),
            method: "GET",
            path: "/documents/\(documentId)",
            retryConfig: Metadata.retryConfig(for: "GetDocument")
        )
    }

    public func list(vaultId: Int, options: ListDocumentOptions? = nil) async throws -> ListResult<Document> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Documents", operation: "ListDocuments", resourceType: "document", isMutation: false, resourceId: vaultId),
            path: "/vaults/\(vaultId)/documents.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListDocuments")
        )
    }

    public func replace(documentId: Int, req: ReplaceDocumentRequest) async throws -> Document {
        return try await request(
            OperationInfo(service: "Documents", operation: "ReplaceDocument", resourceType: "document", isMutation: true, resourceId: documentId),
            method: "PUT",
            path: "/documents/\(documentId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "ReplaceDocument")
        )
    }
}
