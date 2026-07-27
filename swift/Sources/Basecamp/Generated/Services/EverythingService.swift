// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct EverythingBoostsEverythingOptions: Sendable {
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingCheckinsEverythingOptions: Sendable {
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingCommentsEverythingOptions: Sendable {
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingFilesEverythingOptions: Sendable {
    public var kind: String?
    public var peopleIds: [Int]?
    public var page: Int?
    public var maxItems: Int?

    public init(
        kind: String? = nil,
        peopleIds: [Int]? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.kind = kind
        self.peopleIds = peopleIds
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingForwardsEverythingOptions: Sendable {
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingMessagesEverythingOptions: Sendable {
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class EverythingService: BaseService, @unchecked Sendable {
    public func everythingBoosts(options: EverythingBoostsEverythingOptions? = nil) async throws -> ListResult<EverythingBoost> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingBoosts", resourceType: "everything_boost", isMutation: false),
            path: "/boosts.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingBoosts")
        )
    }

    public func everythingCheckins(options: EverythingCheckinsEverythingOptions? = nil) async throws -> ListResult<Recording> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingCheckins", resourceType: "everything_checkin", isMutation: false),
            path: "/checkins.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingCheckins")
        )
    }

    public func everythingComments(options: EverythingCommentsEverythingOptions? = nil) async throws -> ListResult<Recording> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingComments", resourceType: "everything_comment", isMutation: false),
            path: "/comments.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingComments")
        )
    }

    public func everythingFiles(options: EverythingFilesEverythingOptions? = nil) async throws -> ListResult<EverythingFile> {
        var queryItems: [URLQueryItem] = []
        if let kind = options?.kind {
            queryItems.append(URLQueryItem(name: "kind", value: kind))
        }
        if let peopleIds = options?.peopleIds {
            for item in peopleIds {
                queryItems.append(URLQueryItem(name: "people_ids[]", value: String(item)))
            }
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingFiles", resourceType: "everything_file", isMutation: false),
            path: "/files.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingFiles")
        )
    }

    public func everythingForwards(options: EverythingForwardsEverythingOptions? = nil) async throws -> ListResult<Recording> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingForwards", resourceType: "everything_forward", isMutation: false),
            path: "/forwards.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingForwards")
        )
    }

    public func everythingMessages(options: EverythingMessagesEverythingOptions? = nil) async throws -> ListResult<Recording> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingMessages", resourceType: "everything_message", isMutation: false),
            path: "/messages.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingMessages")
        )
    }

    public func everythingOverdueCards() async throws -> [Card] {
        return try await request(
            OperationInfo(service: "Everything", operation: "GetEverythingOverdueCards", resourceType: "everything_overdue_card", isMutation: false),
            method: "GET",
            path: "/cards/overdue.json",
            retryConfig: Metadata.retryConfig(for: "GetEverythingOverdueCards")
        )
    }

    public func everythingOverdueTodos() async throws -> [Todo] {
        return try await request(
            OperationInfo(service: "Everything", operation: "GetEverythingOverdueTodos", resourceType: "everything_overdue_todo", isMutation: false),
            method: "GET",
            path: "/todos/overdue.json",
            retryConfig: Metadata.retryConfig(for: "GetEverythingOverdueTodos")
        )
    }
}
