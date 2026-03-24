// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListClientApprovalOptions: Sendable {
    public var sort: String?
    public var direction: String?
    public var maxItems: Int?

    public init(sort: String? = nil, direction: String? = nil, maxItems: Int? = nil) {
        self.sort = sort
        self.direction = direction
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

    public func list(options: ListClientApprovalOptions? = nil) async throws -> ListResult<ClientApproval> {
        var queryItems: [URLQueryItem] = []
        if let sort = options?.sort {
            queryItems.append(URLQueryItem(name: "sort", value: sort))
        }
        if let direction = options?.direction {
            queryItems.append(URLQueryItem(name: "direction", value: direction))
        }
        return try await requestPaginated(
            OperationInfo(service: "ClientApprovals", operation: "ListClientApprovals", resourceType: "client_approval", isMutation: false),
            path: "/client/approvals.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListClientApprovals")
        )
    }
}
