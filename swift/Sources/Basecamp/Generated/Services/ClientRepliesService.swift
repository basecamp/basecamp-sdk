// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListClientReplyOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class ClientRepliesService: BaseService, @unchecked Sendable {
    public func get(recordingId: Int, replyId: Int) async throws -> ClientReply {
        return try await request(
            OperationInfo(service: "ClientReplies", operation: "GetClientReply", resourceType: "client_reply", isMutation: false, resourceId: replyId),
            method: "GET",
            path: "/client/recordings/\(recordingId)/replies/\(replyId)",
            retryConfig: Metadata.retryConfig(for: "GetClientReply")
        )
    }

    public func list(recordingId: Int, options: ListClientReplyOptions? = nil) async throws -> ListResult<ClientReply> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "ClientReplies", operation: "ListClientReplies", resourceType: "client_reply", isMutation: false, resourceId: recordingId),
            path: "/client/recordings/\(recordingId)/replies.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListClientReplies")
        )
    }
}
