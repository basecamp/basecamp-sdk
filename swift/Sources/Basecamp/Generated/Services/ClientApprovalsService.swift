// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListClientApprovalOptions: Sendable {
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


public final class ClientApprovalsService: BaseService, @unchecked Sendable {
    public func get(approvalId: Int) async throws -> ClientApproval {
        return try await request(
            OperationInfo(service: "ClientApprovals", operation: "GetClientApproval", resourceType: "client_approval", isMutation: false, resourceId: approvalId),
            method: "GET",
            path: "/client/approvals/\(approvalId)",
            retryConfig: Metadata.retryConfig(for: "GetClientApproval")
        )
    }

    public func list(bucketId: Int, options: ListClientApprovalOptions? = nil) async throws -> ListResult<ClientApproval> {
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
            OperationInfo(service: "ClientApprovals", operation: "ListClientApprovals", resourceType: "client_approval", isMutation: false, projectId: bucketId),
            path: "/buckets/\(bucketId)/client/approvals.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListClientApprovals")
        )
    }
}
