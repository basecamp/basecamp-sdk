// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListClientCorrespondenceOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class ClientCorrespondencesService: BaseService, @unchecked Sendable {
    public func get(projectId: Int, correspondenceId: Int) async throws -> ClientCorrespondence {
        return try await request(
            OperationInfo(service: "ClientCorrespondences", operation: "GetClientCorrespondence", resourceType: "client_correspondence", isMutation: false, projectId: projectId, resourceId: correspondenceId),
            method: "GET",
            path: "/buckets/\(projectId)/client/correspondences/\(correspondenceId)",
            retryConfig: Metadata.retryConfig(for: "GetClientCorrespondence")
        )
    }

    public func list(projectId: Int, options: ListClientCorrespondenceOptions? = nil) async throws -> ListResult<ClientCorrespondence> {
        return try await requestPaginated(
            OperationInfo(service: "ClientCorrespondences", operation: "ListClientCorrespondences", resourceType: "client_correspondence", isMutation: false, projectId: projectId),
            path: "/buckets/\(projectId)/client/correspondences.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListClientCorrespondences")
        )
    }
}
