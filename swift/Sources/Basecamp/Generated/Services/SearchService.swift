// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct SearchSearchOptions: Sendable {
    public var typeNames: [String]?
    public var bucketIds: [Int]?
    public var creatorIds: [Int]?
    public var fileType: String?
    public var excludeChat: Bool?
    public var since: String?
    public var sort: String?
    public var type: String?
    public var bucketId: Int?
    public var creatorId: Int?
    public var maxItems: Int?

    public init(
        typeNames: [String]? = nil,
        bucketIds: [Int]? = nil,
        creatorIds: [Int]? = nil,
        fileType: String? = nil,
        excludeChat: Bool? = nil,
        since: String? = nil,
        sort: String? = nil,
        type: String? = nil,
        bucketId: Int? = nil,
        creatorId: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.typeNames = typeNames
        self.bucketIds = bucketIds
        self.creatorIds = creatorIds
        self.fileType = fileType
        self.excludeChat = excludeChat
        self.since = since
        self.sort = sort
        self.type = type
        self.bucketId = bucketId
        self.creatorId = creatorId
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

    public func search(q: String, options: SearchSearchOptions? = nil) async throws -> ListResult<SearchResult> {
        var queryItems: [URLQueryItem] = []
        queryItems.append(URLQueryItem(name: "q", value: q))
        if let typeNames = options?.typeNames {
            for item in typeNames {
                queryItems.append(URLQueryItem(name: "type_names[]", value: item))
            }
        }
        if let bucketIds = options?.bucketIds {
            for item in bucketIds {
                queryItems.append(URLQueryItem(name: "bucket_ids[]", value: String(item)))
            }
        }
        if let creatorIds = options?.creatorIds {
            for item in creatorIds {
                queryItems.append(URLQueryItem(name: "creator_ids[]", value: String(item)))
            }
        }
        if let fileType = options?.fileType {
            queryItems.append(URLQueryItem(name: "file_type", value: fileType))
        }
        if let excludeChat = options?.excludeChat {
            queryItems.append(URLQueryItem(name: "exclude_chat", value: String(excludeChat)))
        }
        if let since = options?.since {
            queryItems.append(URLQueryItem(name: "since", value: since))
        }
        if let sort = options?.sort {
            queryItems.append(URLQueryItem(name: "sort", value: sort))
        }
        if let type = options?.type {
            queryItems.append(URLQueryItem(name: "type", value: type))
        }
        if let bucketId = options?.bucketId {
            queryItems.append(URLQueryItem(name: "bucket_id", value: String(bucketId)))
        }
        if let creatorId = options?.creatorId {
            queryItems.append(URLQueryItem(name: "creator_id", value: String(creatorId)))
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
