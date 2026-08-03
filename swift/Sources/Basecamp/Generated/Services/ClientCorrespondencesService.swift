// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListClientCorrespondenceOptions: Sendable {
    /// created_at|updated_at
    public var sort: String?
    /// asc|desc
    public var direction: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        sort: String? = nil,
        direction: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.sort = sort
        self.direction = direction
        self.page = page
        self.maxItems = maxItems
    }
}


public final class ClientCorrespondencesService: BaseService, @unchecked Sendable {
    public func get(correspondenceId: Int) async throws -> ClientCorrespondence {
        return try await request(
            OperationInfo(service: "ClientCorrespondences", operation: "GetClientCorrespondence", resourceType: "client_correspondence", isMutation: false, resourceId: correspondenceId),
            method: "GET",
            path: "/client/correspondences/\(correspondenceId)",
            retryConfig: Metadata.retryConfig(for: "GetClientCorrespondence")
        )
    }

    public func list(bucketId: Int, options: ListClientCorrespondenceOptions? = nil) async throws -> ListResult<ClientCorrespondence> {
        var queryItems: [URLQueryItem] = []
        if let sort = options?.sort {
            queryItems.append(URLQueryItem(name: "sort", value: sort))
        }
        if let direction = options?.direction {
            queryItems.append(URLQueryItem(name: "direction", value: direction))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "ClientCorrespondences", operation: "ListClientCorrespondences", resourceType: "client_correspondence", isMutation: false, projectId: bucketId),
            path: "/buckets/\(bucketId)/client/correspondences.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListClientCorrespondences")
        )
    }
}
