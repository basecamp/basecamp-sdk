// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct AssignedReportOptions: Sendable {
    public var groupBy: String?

    public init(groupBy: String? = nil) {
        self.groupBy = groupBy
    }
}

public struct MyAssignmentsDueReportOptions: Sendable {
    public var scope: String?

    public init(scope: String? = nil) {
        self.scope = scope
    }
}

public struct PersonProgressReportOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ProgressReportOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct UpcomingReportOptions: Sendable {
    public var windowStartsOn: String?
    public var windowEndsOn: String?

    public init(windowStartsOn: String? = nil, windowEndsOn: String? = nil) {
        self.windowStartsOn = windowStartsOn
        self.windowEndsOn = windowEndsOn
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

    public func myAssignments() async throws -> GetMyAssignmentsResponseContent {
        return try await request(
            OperationInfo(service: "Reports", operation: "GetMyAssignments", resourceType: "my_assignment", isMutation: false),
            method: "GET",
            path: "/my/assignments.json",
            retryConfig: Metadata.retryConfig(for: "GetMyAssignments")
        )
    }

    public func myAssignmentsCompleted() async throws -> [MyAssignment] {
        return try await request(
            OperationInfo(service: "Reports", operation: "GetMyAssignmentsCompleted", resourceType: "my_assignments_completed", isMutation: false),
            method: "GET",
            path: "/my/assignments/completed.json",
            retryConfig: Metadata.retryConfig(for: "GetMyAssignmentsCompleted")
        )
    }

    public func myAssignmentsDue(options: MyAssignmentsDueReportOptions? = nil) async throws -> [MyAssignment] {
        var queryItems: [URLQueryItem] = []
        if let scope = options?.scope {
            queryItems.append(URLQueryItem(name: "scope", value: scope))
        }
        return try await request(
            OperationInfo(service: "Reports", operation: "GetMyAssignmentsDue", resourceType: "my_assignments_due", isMutation: false),
            method: "GET",
            path: "/my/assignments/due.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetMyAssignmentsDue")
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
        let (wrapperData, items): (Data, ListResult<TimelineEvent>) = try await requestPaginatedWrapped(
            OperationInfo(service: "Reports", operation: "GetPersonProgress", resourceType: "person_progress", isMutation: false, resourceId: personId),
            path: "/reports/users/progress/\(personId).json",
            itemsKey: "events",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetPersonProgress")
        )
        struct Wrapper: Decodable {
            let person: Person
        }
        let wrapper = try Self.decoder.decode(Wrapper.self, from: wrapperData)
        return PersonProgressResult(events: items, person: wrapper.person)
    }

    public func progress(options: ProgressReportOptions? = nil) async throws -> ListResult<TimelineEvent> {
        return try await requestPaginated(
            OperationInfo(service: "Reports", operation: "GetProgressReport", resourceType: "progress_report", isMutation: false),
            path: "/reports/progress.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetProgressReport")
        )
    }

    public func upcoming(options: UpcomingReportOptions? = nil) async throws -> GetUpcomingScheduleResponseContent {
        var queryItems: [URLQueryItem] = []
        if let windowStartsOn = options?.windowStartsOn {
            queryItems.append(URLQueryItem(name: "window_starts_on", value: windowStartsOn))
        }
        if let windowEndsOn = options?.windowEndsOn {
            queryItems.append(URLQueryItem(name: "window_ends_on", value: windowEndsOn))
        }
        return try await request(
            OperationInfo(service: "Reports", operation: "GetUpcomingSchedule", resourceType: "upcoming_schedule", isMutation: false),
            method: "GET",
            path: "/reports/schedules/upcoming.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetUpcomingSchedule")
        )
    }
}
