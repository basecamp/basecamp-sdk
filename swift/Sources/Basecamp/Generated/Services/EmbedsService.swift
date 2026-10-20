// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class EmbedsService: BaseService, @unchecked Sendable {
    public func create(req: CreateEmbedRequest) async throws -> Embed {
        return try await request(
            OperationInfo(service: "Embeds", operation: "CreateEmbed", resourceType: "embed", isMutation: true),
            method: "POST",
            path: "/embeds.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateEmbed")
        )
    }

    public func get(sgid: String) async throws -> Embed {
        return try await request(
            OperationInfo(service: "Embeds", operation: "GetEmbed", resourceType: "embed", isMutation: false),
            method: "GET",
            path: "/embeds/\(pathSegment(sgid))",
            retryConfig: Metadata.retryConfig(for: "GetEmbed")
        )
    }
}
