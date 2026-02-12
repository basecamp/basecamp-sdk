// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct SearchSearchOptions: Sendable {
    public var sort: String?
    public var page: Int?
    public var maxItems: Int?

    public init(sort: String? = nil, page: Int? = nil, maxItems: Int? = nil) {
        self.sort = sort
        self.page = page
        self.maxItems = maxItems
    }
}


public final class SearchService: BaseService, @unchecked Sendable {
    public func metadata() async throws -> SearchMetadata {
        return try await request(
            OperationInfo(service: "Search", operation: "GetSearchMetadata", resourceType: "search_metadata", isMutation: false),
            method: "GET",
            path: "/searches/metadata.json",
            retryConfig: Metadata.retryConfig(for: "GetSearchMetadata")
        )
    }

    public func search(query: String, options: SearchSearchOptions? = nil) async throws -> ListResult<SearchResult> {
        var queryItems: [URLQueryItem] = []
        queryItems.append(URLQueryItem(name: "query", value: query))
        if let sort = options?.sort {
            queryItems.append(URLQueryItem(name: "sort", value: sort))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Search", operation: "Search", resourceType: "resource", isMutation: false),
            path: "/search.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "Search")
        )
    }
}
