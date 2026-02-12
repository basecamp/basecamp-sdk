// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListVersionsUploadOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListUploadOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class UploadsService: BaseService, @unchecked Sendable {
    public func create(projectId: Int, vaultId: Int, req: CreateUploadRequest) async throws -> Upload {
        return try await request(
            OperationInfo(service: "Uploads", operation: "CreateUpload", resourceType: "upload", isMutation: true, projectId: projectId, resourceId: vaultId),
            method: "POST",
            path: "/buckets/\(projectId)/vaults/\(vaultId)/uploads.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateUpload")
        )
    }

    public func get(projectId: Int, uploadId: Int) async throws -> Upload {
        return try await request(
            OperationInfo(service: "Uploads", operation: "GetUpload", resourceType: "upload", isMutation: false, projectId: projectId, resourceId: uploadId),
            method: "GET",
            path: "/buckets/\(projectId)/uploads/\(uploadId)",
            retryConfig: Metadata.retryConfig(for: "GetUpload")
        )
    }

    public func listVersions(projectId: Int, uploadId: Int, options: ListVersionsUploadOptions? = nil) async throws -> ListResult<Upload> {
        return try await requestPaginated(
            OperationInfo(service: "Uploads", operation: "ListUploadVersions", resourceType: "upload_version", isMutation: false, projectId: projectId, resourceId: uploadId),
            path: "/buckets/\(projectId)/uploads/\(uploadId)/versions.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListUploadVersions")
        )
    }

    public func list(projectId: Int, vaultId: Int, options: ListUploadOptions? = nil) async throws -> ListResult<Upload> {
        return try await requestPaginated(
            OperationInfo(service: "Uploads", operation: "ListUploads", resourceType: "upload", isMutation: false, projectId: projectId, resourceId: vaultId),
            path: "/buckets/\(projectId)/vaults/\(vaultId)/uploads.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListUploads")
        )
    }

    public func update(projectId: Int, uploadId: Int, req: UpdateUploadRequest) async throws -> Upload {
        return try await request(
            OperationInfo(service: "Uploads", operation: "UpdateUpload", resourceType: "upload", isMutation: true, projectId: projectId, resourceId: uploadId),
            method: "PUT",
            path: "/buckets/\(projectId)/uploads/\(uploadId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateUpload")
        )
    }
}
