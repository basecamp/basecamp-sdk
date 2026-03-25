// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListClientCorrespondenceOptions: Sendable {
    public var sort: String?
    public var direction: String?
    public var maxItems: Int?

    public init(sort: String? = nil, direction: String? = nil, maxItems: Int? = nil) {
        self.sort = sort
        self.direction = direction
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

    public func list(options: ListClientCorrespondenceOptions? = nil) async throws -> ListResult<ClientCorrespondence> {
        var queryItems: [URLQueryItem] = []
        if let sort = options?.sort {
            queryItems.append(URLQueryItem(name: "sort", value: sort))
        }
        if let direction = options?.direction {
            queryItems.append(URLQueryItem(name: "direction", value: direction))
        }
        return try await requestPaginated(
            OperationInfo(service: "ClientCorrespondences", operation: "ListClientCorrespondences", resourceType: "client_correspondence", isMutation: false),
            path: "/client/correspondences.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListClientCorrespondences")
        )
    }
}
