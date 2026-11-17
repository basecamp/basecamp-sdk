// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct AssignedReportOptions: Sendable {
    /// Group by "bucket" or "date"
    public var groupBy: String?
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(groupBy: String? = nil, page: Int? = nil, maxItems: Int? = nil) {
        self.groupBy = groupBy
        self.page = page
        self.maxItems = maxItems
    }
}

public struct OverdueReportOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. This operation is not auto-paginated in any SDK, so a page is returned as asked for and later pages are not followed.
    public var page: Int?

    public init(page: Int? = nil) {
        self.page = page
    }
}

public struct PersonProgressReportOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct ProgressReportOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public struct AssignedTodosResult: Sendable {
    public let groupedBy: String
    public let person: Person
    public let todos: ListResult<Todo>
}

public struct PersonProgressResult: Sendable {
    public let events: ListResult<TimelineEvent>
    public let person: Person
}


public final class ReportsService: BaseService, @unchecked Sendable {
    public func assigned(personId: Int, options: AssignedReportOptions? = nil) async throws -> AssignedTodosResult {
        var queryItems: [URLQueryItem] = []
        if let groupBy = options?.groupBy {
            queryItems.append(URLQueryItem(name: "group_by", value: groupBy))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        struct Wrapper: Decodable {
            let groupedBy: String
            let person: Person
        }
        return try await requestPaginatedWrapped(
            OperationInfo(service: "Reports", operation: "GetAssignedTodos", resourceType: "assigned_todo", isMutation: false, resourceId: personId),
            path: "/reports/todos/assigned/\(personId)",
            itemsKey: "todos",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetAssignedTodos")
        ) { (wrapperData: Data, items: ListResult<Todo>) -> AssignedTodosResult in
            let wrapper = try Self.decoder.decode(Wrapper.self, from: wrapperData)
            return AssignedTodosResult(groupedBy: wrapper.groupedBy, person: wrapper.person, todos: items)
        }
    }

    public func overdue(options: OverdueReportOptions? = nil) async throws -> GetOverdueTodosResponseContent {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await request(
            OperationInfo(service: "Reports", operation: "GetOverdueTodos", resourceType: "overdue_todo", isMutation: false),
            method: "GET",
            path: "/reports/todos/overdue.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetOverdueTodos")
        )
    }

    public func personProgress(personId: Int, options: PersonProgressReportOptions? = nil) async throws -> PersonProgressResult {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        struct Wrapper: Decodable {
            let person: Person
        }
        return try await requestPaginatedWrapped(
            OperationInfo(service: "Reports", operation: "GetPersonProgress", resourceType: "person_progress", isMutation: false, resourceId: personId),
            path: "/reports/users/progress/\(personId).json",
            itemsKey: "events",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetPersonProgress")
        ) { (wrapperData: Data, items: ListResult<TimelineEvent>) -> PersonProgressResult in
            let wrapper = try Self.decoder.decode(Wrapper.self, from: wrapperData)
            return PersonProgressResult(events: items, person: wrapper.person)
        }
    }

    public func progress(options: ProgressReportOptions? = nil) async throws -> ListResult<TimelineEvent> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Reports", operation: "GetProgressReport", resourceType: "progress_report", isMutation: false),
            path: "/reports/progress.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetProgressReport")
        )
    }

    public func upcoming(windowStartsOn: String, windowEndsOn: String) async throws -> GetUpcomingScheduleResponseContent {
        var queryItems: [URLQueryItem] = []
        queryItems.append(URLQueryItem(name: "window_starts_on", value: windowStartsOn))
        queryItems.append(URLQueryItem(name: "window_ends_on", value: windowEndsOn))
        return try await request(
            OperationInfo(service: "Reports", operation: "GetUpcomingSchedule", resourceType: "upcoming_schedule", isMutation: false),
            method: "GET",
            path: "/reports/schedules/upcoming.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetUpcomingSchedule")
        )
    }
}
