// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class GoogleDocumentsService: BaseService, @unchecked Sendable {
    public func createGoogleDocument(bucketId: Int, vaultId: Int, req: CreateGoogleDocumentRequest) async throws -> GoogleDocument {
        return try await request(
            OperationInfo(service: "GoogleDocuments", operation: "CreateGoogleDocument", resourceType: "google_document", isMutation: true, projectId: bucketId, resourceId: vaultId),
            method: "POST",
            path: "/buckets/\(bucketId)/vaults/\(vaultId)/google_documents.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateGoogleDocument")
        )
    }

    public func googleDocument(googleDocumentId: Int) async throws -> GoogleDocument {
        return try await request(
            OperationInfo(service: "GoogleDocuments", operation: "GetGoogleDocument", resourceType: "google_document", isMutation: false, resourceId: googleDocumentId),
            method: "GET",
            path: "/google_documents/\(googleDocumentId)",
            retryConfig: Metadata.retryConfig(for: "GetGoogleDocument")
        )
    }

    public func updateGoogleDocument(googleDocumentId: Int, req: UpdateGoogleDocumentRequest) async throws -> GoogleDocument {
        return try await request(
            OperationInfo(service: "GoogleDocuments", operation: "UpdateGoogleDocument", resourceType: "google_document", isMutation: true, resourceId: googleDocumentId),
            method: "PUT",
            path: "/google_documents/\(googleDocumentId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateGoogleDocument")
        )
    }
}
