// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class FoldersService: BaseService, @unchecked Sendable {
    public func createFolder(req: CreateFolderRequest) async throws -> FolderWithProjects {
        return try await request(
            OperationInfo(service: "Folders", operation: "CreateFolder", resourceType: "folder", isMutation: true),
            method: "POST",
            path: "/stacks.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateFolder")
        )
    }

    public func deleteFolder(folderId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Folders", operation: "DeleteFolder", resourceType: "folder", isMutation: true, resourceId: folderId),
            method: "DELETE",
            path: "/stacks/\(folderId)",
            retryConfig: Metadata.retryConfig(for: "DeleteFolder")
        )
    }

    public func getFolder(folderId: Int) async throws -> FolderWithProjects {
        return try await request(
            OperationInfo(service: "Folders", operation: "GetFolder", resourceType: "folder", isMutation: false, resourceId: folderId),
            method: "GET",
            path: "/stacks/\(folderId)",
            retryConfig: Metadata.retryConfig(for: "GetFolder")
        )
    }

    public func listFolders() async throws -> [Folder] {
        return try await request(
            OperationInfo(service: "Folders", operation: "ListFolders", resourceType: "folder", isMutation: false),
            method: "GET",
            path: "/stacks.json",
            retryConfig: Metadata.retryConfig(for: "ListFolders")
        )
    }

    public func updateFolder(folderId: Int, req: UpdateFolderRequest) async throws -> FolderWithProjects {
        return try await request(
            OperationInfo(service: "Folders", operation: "UpdateFolder", resourceType: "folder", isMutation: true, resourceId: folderId),
            method: "PUT",
            path: "/stacks/\(folderId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateFolder")
        )
    }
}
