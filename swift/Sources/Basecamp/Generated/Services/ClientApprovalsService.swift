// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListClientApprovalOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class ClientApprovalsService: BaseService, @unchecked Sendable {
    public func get(projectId: Int, approvalId: Int) async throws -> ClientApproval {
        return try await request(
            OperationInfo(service: "ClientApprovals", operation: "GetClientApproval", resourceType: "client_approval", isMutation: false, projectId: projectId, resourceId: approvalId),
            method: "GET",
            path: "/buckets/\(projectId)/client/approvals/\(approvalId)",
            retryConfig: Metadata.retryConfig(for: "GetClientApproval")
        )
    }

    public func list(projectId: Int, options: ListClientApprovalOptions? = nil) async throws -> ListResult<ClientApproval> {
        return try await requestPaginated(
            OperationInfo(service: "ClientApprovals", operation: "ListClientApprovals", resourceType: "client_approval", isMutation: false, projectId: projectId),
            path: "/buckets/\(projectId)/client/approvals.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListClientApprovals")
        )
    }
}
