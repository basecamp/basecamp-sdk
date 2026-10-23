// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct AssignedReportOptions: Sendable {
    /// Group by "bucket" or "date"
    public var groupBy: String?

    public init(groupBy: String? = nil) {
        self.groupBy = groupBy
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
    /// The day to read (YYYY-MM-DD). The feed serves one day per request and defaults to today, so step this back a day at a time to read history; `page` pages within the day. The server reads an unparseable date as today.
    public var date: String?
    /// Only events in these projects. When omitted, the server applies the project filter the caller last saved in the web's activity view, not "all projects". If none of the IDs is a project the caller can access, the response is empty and the server clears that saved filter, so send only IDs the API just returned.
    public var projectIds: [Int]?
    /// Only events created by these people. When omitted, the server applies the people filter the caller last saved in the web's activity view, not "everyone". If none of the IDs is a person the caller can see, the response is empty and the server clears that saved filter, so send only IDs the API just returned.
    public var peopleIds: [Int]?
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(
        date: String? = nil,
        projectIds: [Int]? = nil,
        peopleIds: [Int]? = nil,
        page: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.date = date
        self.projectIds = projectIds
        self.peopleIds = peopleIds
        self.page = page
        self.maxItems = maxItems
    }
}


public struct PersonProgressResult: Sendable {
    public let events: ListResult<TimelineEvent>
    public let person: Person
}


public final class ReportsService: BaseService, @unchecked Sendable {
    public func assigned(personId: Int, options: AssignedReportOptions? = nil) async throws -> GetAssignedTodosResponseContent {
        var queryItems: [URLQueryItem] = []
        if let groupBy = options?.groupBy {
            queryItems.append(URLQueryItem(name: "group_by", value: groupBy))
        }
        return try await request(
            OperationInfo(service: "Reports", operation: "GetAssignedTodos", resourceType: "assigned_todo", isMutation: false, resourceId: personId),
            method: "GET",
            path: "/reports/todos/assigned/\(personId)" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetAssignedTodos")
        )
    }

    public func overdue() async throws -> GetOverdueTodosResponseContent {
        return try await request(
            OperationInfo(service: "Reports", operation: "GetOverdueTodos", resourceType: "overdue_todo", isMutation: false),
            method: "GET",
            path: "/reports/todos/overdue.json",
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
        if let date = options?.date {
            queryItems.append(URLQueryItem(name: "date", value: date))
        }
        if let projectIds = options?.projectIds {
            for item in projectIds {
                queryItems.append(URLQueryItem(name: "project_ids[]", value: String(item)))
            }
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
