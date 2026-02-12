// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class AttachmentsService: BaseService, @unchecked Sendable {
    public func create(data: Data, contentType: String, name: String) async throws -> CreateAttachmentResponseContent {
        var queryItems: [URLQueryItem] = []
        queryItems.append(URLQueryItem(name: "name", value: name))
        return try await request(
            OperationInfo(service: "Attachments", operation: "CreateAttachment", resourceType: "attachment", isMutation: true),
            method: "POST",
            path: "/attachments.json" + queryString(queryItems),
            body: data,
            contentType: contentType,
            retryConfig: Metadata.retryConfig(for: "CreateAttachment")
        )
    }
}
