// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class CloudFilesService: BaseService, @unchecked Sendable {
    public func createCloudFile(bucketId: Int, vaultId: Int, req: CreateCloudFileRequest) async throws -> CloudFile {
        return try await request(
            OperationInfo(service: "CloudFiles", operation: "CreateCloudFile", resourceType: "cloud_file", isMutation: true, projectId: bucketId, resourceId: vaultId),
            method: "POST",
            path: "/buckets/\(bucketId)/vaults/\(vaultId)/cloud_files.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateCloudFile")
        )
    }

    public func cloudFile(cloudFileId: Int) async throws -> CloudFile {
        return try await request(
            OperationInfo(service: "CloudFiles", operation: "GetCloudFile", resourceType: "cloud_file", isMutation: false, resourceId: cloudFileId),
            method: "GET",
            path: "/cloud_files/\(cloudFileId)",
            retryConfig: Metadata.retryConfig(for: "GetCloudFile")
        )
    }

    public func updateCloudFile(cloudFileId: Int, req: UpdateCloudFileRequest) async throws -> CloudFile {
        return try await request(
            OperationInfo(service: "CloudFiles", operation: "UpdateCloudFile", resourceType: "cloud_file", isMutation: true, resourceId: cloudFileId),
            method: "PUT",
            path: "/cloud_files/\(cloudFileId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateCloudFile")
        )
    }
}
