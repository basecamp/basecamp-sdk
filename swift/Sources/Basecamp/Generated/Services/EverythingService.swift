// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct EverythingCheckinsEverythingOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingCommentsEverythingOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingCompletedCardsEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        assigneeIds: [Int]? = nil,
        due: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.due = due
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingCompletedTodosEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        assigneeIds: [Int]? = nil,
        due: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.due = due
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingFilesEverythingOptions: Sendable {
    /// Filter by file kind: all (default), images, pdfs, documents, or videos.
    public var kind: String?
    /// Restrict to files created by the given people (repeatable).
    public var peopleIds: [Int]?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
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
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingMessagesEverythingOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingNoDueDateCardsEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        assigneeIds: [Int]? = nil,
        due: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.due = due
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingNoDueDateTodosEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        assigneeIds: [Int]? = nil,
        due: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.due = due
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingNotNowCardsEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        assigneeIds: [Int]? = nil,
        due: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.due = due
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingOpenCardsEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        assigneeIds: [Int]? = nil,
        due: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.due = due
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingOpenTodosEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        assigneeIds: [Int]? = nil,
        due: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.due = due
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingOverdueCardsEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?

    public init(assigneeIds: [Int]? = nil, due: String? = nil) {
        self.assigneeIds = assigneeIds
        self.due = due
    }
}

public struct EverythingOverdueTodosEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?

    public init(assigneeIds: [Int]? = nil, due: String? = nil) {
        self.assigneeIds = assigneeIds
        self.due = due
    }
}

public struct EverythingUnassignedCardsEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        assigneeIds: [Int]? = nil,
        due: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.due = due
        self.page = page
        self.maxItems = maxItems
    }
}

public struct EverythingUnassignedTodosEverythingOptions: Sendable {
    /// Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered.
    public var assigneeIds: [Int]?
    /// Filter by due date: with, without, or overdue. Unrecognized values are ignored.
    public var due: String?
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        assigneeIds: [Int]? = nil,
        due: String? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.due = due
        self.page = page
        self.maxItems = maxItems
    }
}


public final class EverythingService: BaseService, @unchecked Sendable {
    public func everythingCheckins(options: EverythingCheckinsEverythingOptions? = nil) async throws -> ListResult<Recording> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingCheckins", resourceType: "everything_checkin", isMutation: false),
            path: "/checkins.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
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
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingComments")
        )
    }

    public func everythingCompletedCards(options: EverythingCompletedCardsEverythingOptions? = nil) async throws -> ListResult<BucketCardsGroup> {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingCompletedCards", resourceType: "everything_completed_card", isMutation: false),
            path: "/cards/completed.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingCompletedCards")
        )
    }

    public func everythingCompletedTodos(options: EverythingCompletedTodosEverythingOptions? = nil) async throws -> ListResult<BucketTodosGroup> {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingCompletedTodos", resourceType: "everything_completed_todo", isMutation: false),
            path: "/todos/completed.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingCompletedTodos")
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
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
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
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
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
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingMessages")
        )
    }

    public func everythingNoDueDateCards(options: EverythingNoDueDateCardsEverythingOptions? = nil) async throws -> ListResult<BucketCardsGroup> {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingNoDueDateCards", resourceType: "everything_no_due_date_card", isMutation: false),
            path: "/cards/no_due_date.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingNoDueDateCards")
        )
    }

    public func everythingNoDueDateTodos(options: EverythingNoDueDateTodosEverythingOptions? = nil) async throws -> ListResult<BucketTodosGroup> {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingNoDueDateTodos", resourceType: "everything_no_due_date_todo", isMutation: false),
            path: "/todos/no_due_date.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingNoDueDateTodos")
        )
    }

    public func everythingNotNowCards(options: EverythingNotNowCardsEverythingOptions? = nil) async throws -> ListResult<BucketCardsGroup> {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingNotNowCards", resourceType: "everything_not_now_card", isMutation: false),
            path: "/cards/not_now.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingNotNowCards")
        )
    }

    public func everythingOpenCards(options: EverythingOpenCardsEverythingOptions? = nil) async throws -> ListResult<BucketCardsGroup> {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingOpenCards", resourceType: "everything_open_card", isMutation: false),
            path: "/cards/open.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingOpenCards")
        )
    }

    public func everythingOpenTodos(options: EverythingOpenTodosEverythingOptions? = nil) async throws -> ListResult<BucketTodosGroup> {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingOpenTodos", resourceType: "everything_open_todo", isMutation: false),
            path: "/todos/open.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingOpenTodos")
        )
    }

    public func everythingOverdueCards(options: EverythingOverdueCardsEverythingOptions? = nil) async throws -> [Card] {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        return try await request(
            OperationInfo(service: "Everything", operation: "GetEverythingOverdueCards", resourceType: "everything_overdue_card", isMutation: false),
            method: "GET",
            path: "/cards/overdue.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetEverythingOverdueCards")
        )
    }

    public func everythingOverdueTodos(options: EverythingOverdueTodosEverythingOptions? = nil) async throws -> [Todo] {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        return try await request(
            OperationInfo(service: "Everything", operation: "GetEverythingOverdueTodos", resourceType: "everything_overdue_todo", isMutation: false),
            method: "GET",
            path: "/todos/overdue.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetEverythingOverdueTodos")
        )
    }

    public func everythingUnassignedCards(options: EverythingUnassignedCardsEverythingOptions? = nil) async throws -> ListResult<BucketCardsGroup> {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingUnassignedCards", resourceType: "everything_unassigned_card", isMutation: false),
            path: "/cards/unassigned.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingUnassignedCards")
        )
    }

    public func everythingUnassignedTodos(options: EverythingUnassignedTodosEverythingOptions? = nil) async throws -> ListResult<BucketTodosGroup> {
        var queryItems: [URLQueryItem] = []
        if let assigneeIds = options?.assigneeIds {
            for item in assigneeIds {
                queryItems.append(URLQueryItem(name: "assignee_ids[]", value: String(item)))
            }
        }
        if let due = options?.due {
            queryItems.append(URLQueryItem(name: "due", value: due))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Everything", operation: "GetEverythingUnassignedTodos", resourceType: "everything_unassigned_todo", isMutation: false),
            path: "/todos/unassigned.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetEverythingUnassignedTodos")
        )
    }
}
