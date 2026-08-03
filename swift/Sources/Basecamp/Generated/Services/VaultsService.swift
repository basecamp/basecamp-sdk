// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListVaultOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class VaultsService: BaseService, @unchecked Sendable {
    public func create(vaultId: Int, req: CreateVaultRequest) async throws -> Vault {
        return try await request(
            OperationInfo(service: "Vaults", operation: "CreateVault", resourceType: "vault", isMutation: true, resourceId: vaultId),
            method: "POST",
            path: "/vaults/\(vaultId)/vaults.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateVault")
        )
    }

    public func get(vaultId: Int) async throws -> Vault {
        return try await request(
            OperationInfo(service: "Vaults", operation: "GetVault", resourceType: "vault", isMutation: false, resourceId: vaultId),
            method: "GET",
            path: "/vaults/\(vaultId)",
            retryConfig: Metadata.retryConfig(for: "GetVault")
        )
    }

    public func list(vaultId: Int, options: ListVaultOptions? = nil) async throws -> ListResult<Vault> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Vaults", operation: "ListVaults", resourceType: "vault", isMutation: false, resourceId: vaultId),
            path: "/vaults/\(vaultId)/vaults.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListVaults")
        )
    }

    public func update(vaultId: Int, req: UpdateVaultRequest) async throws -> Vault {
        return try await request(
            OperationInfo(service: "Vaults", operation: "UpdateVault", resourceType: "vault", isMutation: true, resourceId: vaultId),
            method: "PUT",
            path: "/vaults/\(vaultId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateVault")
        )
    }
}
