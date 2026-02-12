// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListVaultOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class VaultsService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, vaultId: Int, req: CreateVaultRequest) async throws -> Vault {
        return try await request(
            OperationInfo(service: "Vaults", operation: "CreateVault", resourceType: "vault", isMutation: true, projectId: projectId, resourceId: vaultId),
            method: "POST",
            path: "/buckets/\(projectId)/vaults/\(vaultId)/vaults.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateVault")
        )
    }

    public func get(projectId: Int, vaultId: Int) async throws -> Vault {
        return try await request(
            OperationInfo(service: "Vaults", operation: "GetVault", resourceType: "vault", isMutation: false, projectId: projectId, resourceId: vaultId),
            method: "GET",
            path: "/buckets/\(projectId)/vaults/\(vaultId)",
            retryConfig: Metadata.retryConfig(for: "GetVault")
        )
    }

    public func list(projectId: Int, vaultId: Int, options: ListVaultOptions? = nil) async throws -> ListResult<Vault> {
        return try await requestPaginated(
            OperationInfo(service: "Vaults", operation: "ListVaults", resourceType: "vault", isMutation: false, projectId: projectId, resourceId: vaultId),
            path: "/buckets/\(projectId)/vaults/\(vaultId)/vaults.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListVaults")
        )
    }

    public func update(projectId: Int, vaultId: Int, req: UpdateVaultRequest) async throws -> Vault {
        return try await request(
            OperationInfo(service: "Vaults", operation: "UpdateVault", resourceType: "vault", isMutation: true, projectId: projectId, resourceId: vaultId),
            method: "PUT",
            path: "/buckets/\(projectId)/vaults/\(vaultId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateVault")
        )
    }
}
