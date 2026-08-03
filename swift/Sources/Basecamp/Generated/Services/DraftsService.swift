// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListMyDraftsDraftOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class DraftsService: BaseService, @unchecked Sendable {
    public func listMyDrafts(options: ListMyDraftsDraftOptions? = nil) async throws -> ListResult<Draft> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Drafts", operation: "ListMyDrafts", resourceType: "my_draft", isMutation: false),
            path: "/my/drafts.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListMyDrafts")
        )
    }
}
