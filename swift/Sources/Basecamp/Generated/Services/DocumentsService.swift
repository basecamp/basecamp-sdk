// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListDocumentOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
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
        return try await requestPaginated(
            OperationInfo(service: "Documents", operation: "ListDocuments", resourceType: "document", isMutation: false, resourceId: vaultId),
            path: "/vaults/\(vaultId)/documents.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListDocuments")
        )
    }

    public func update(documentId: Int, req: UpdateDocumentRequest) async throws -> Document {
        return try await request(
            OperationInfo(service: "Documents", operation: "UpdateDocument", resourceType: "document", isMutation: true, resourceId: documentId),
            method: "PUT",
            path: "/documents/\(documentId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateDocument")
        )
    }
}
