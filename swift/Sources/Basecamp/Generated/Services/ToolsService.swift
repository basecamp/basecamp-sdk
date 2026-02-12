// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class ToolsService: BaseService, @unchecked Sendable {
    public func clone(projectId: Int, req: CloneToolRequest) async throws -> Tool {
        return try await request(
            OperationInfo(service: "Tools", operation: "CloneTool", resourceType: "tool", isMutation: true, projectId: projectId),
            method: "POST",
            path: "/buckets/\(projectId)/dock/tools.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CloneTool")
        )
    }

    public func delete(projectId: Int, toolId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Tools", operation: "DeleteTool", resourceType: "tool", isMutation: true, projectId: projectId, resourceId: toolId),
            method: "DELETE",
            path: "/buckets/\(projectId)/dock/tools/\(toolId)",
            retryConfig: Metadata.retryConfig(for: "DeleteTool")
        )
    }

    public func disable(projectId: Int, toolId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Tools", operation: "DisableTool", resourceType: "tool", isMutation: true, projectId: projectId, resourceId: toolId),
            method: "DELETE",
            path: "/buckets/\(projectId)/recordings/\(toolId)/position.json",
            retryConfig: Metadata.retryConfig(for: "DisableTool")
        )
    }

    public func enable(projectId: Int, toolId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Tools", operation: "EnableTool", resourceType: "tool", isMutation: true, projectId: projectId, resourceId: toolId),
            method: "POST",
            path: "/buckets/\(projectId)/recordings/\(toolId)/position.json",
            retryConfig: Metadata.retryConfig(for: "EnableTool")
        )
    }

    public func get(projectId: Int, toolId: Int) async throws -> Tool {
        return try await request(
            OperationInfo(service: "Tools", operation: "GetTool", resourceType: "tool", isMutation: false, projectId: projectId, resourceId: toolId),
            method: "GET",
            path: "/buckets/\(projectId)/dock/tools/\(toolId)",
            retryConfig: Metadata.retryConfig(for: "GetTool")
        )
    }

    public func reposition(projectId: Int, toolId: Int, req: RepositionToolRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Tools", operation: "RepositionTool", resourceType: "tool", isMutation: true, projectId: projectId, resourceId: toolId),
            method: "PUT",
            path: "/buckets/\(projectId)/recordings/\(toolId)/position.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "RepositionTool")
        )
    }

    public func update(projectId: Int, toolId: Int, req: UpdateToolRequest) async throws -> Tool {
        return try await request(
            OperationInfo(service: "Tools", operation: "UpdateTool", resourceType: "tool", isMutation: true, projectId: projectId, resourceId: toolId),
            method: "PUT",
            path: "/buckets/\(projectId)/dock/tools/\(toolId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateTool")
        )
    }
}
