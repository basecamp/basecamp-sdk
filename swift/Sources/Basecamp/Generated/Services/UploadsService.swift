// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListVersionsUploadOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListUploadOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class UploadsService: BaseService, @unchecked Sendable {
    public func create(vaultId: Int, req: CreateUploadRequest) async throws -> Upload {
        return try await request(
            OperationInfo(service: "Uploads", operation: "CreateUpload", resourceType: "upload", isMutation: true, resourceId: vaultId),
            method: "POST",
            path: "/vaults/\(vaultId)/uploads.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateUpload")
        )
    }

    public func get(uploadId: Int) async throws -> Upload {
        return try await request(
            OperationInfo(service: "Uploads", operation: "GetUpload", resourceType: "upload", isMutation: false, resourceId: uploadId),
            method: "GET",
            path: "/uploads/\(uploadId)",
            retryConfig: Metadata.retryConfig(for: "GetUpload")
        )
    }

    public func listVersions(uploadId: Int, options: ListVersionsUploadOptions? = nil) async throws -> ListResult<Upload> {
        return try await requestPaginated(
            OperationInfo(service: "Uploads", operation: "ListUploadVersions", resourceType: "upload_version", isMutation: false, resourceId: uploadId),
            path: "/uploads/\(uploadId)/versions.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListUploadVersions")
        )
    }

    public func list(vaultId: Int, options: ListUploadOptions? = nil) async throws -> ListResult<Upload> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Uploads", operation: "ListUploads", resourceType: "upload", isMutation: false, resourceId: vaultId),
            path: "/vaults/\(vaultId)/uploads.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListUploads")
        )
    }

    public func update(uploadId: Int, req: UpdateUploadRequest) async throws -> Upload {
        return try await request(
            OperationInfo(service: "Uploads", operation: "UpdateUpload", resourceType: "upload", isMutation: true, resourceId: uploadId),
            method: "PUT",
            path: "/uploads/\(uploadId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateUpload")
        )
    }
}
