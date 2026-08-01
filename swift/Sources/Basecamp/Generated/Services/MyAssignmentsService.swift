// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyDueAssignmentsMyAssignmentOptions: Sendable {
    /// Filter by due date range: overdue, due_today, due_tomorrow, due_later_this_week, due_next_week, due_later
    public var scope: String?

    public init(scope: String? = nil) {
        self.scope = scope
    }
}


public final class MyAssignmentsService: BaseService, @unchecked Sendable {
    public func deprioritizeAssignment(recordingId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "MyAssignments", operation: "DeprioritizeAssignment", resourceType: "resource", isMutation: true, resourceId: recordingId),
            method: "DELETE",
            path: "/my/priorities/\(recordingId)",
            retryConfig: Metadata.retryConfig(for: "DeprioritizeAssignment")
        )
    }

    public func myAssignments() async throws -> GetMyAssignmentsResponseContent {
        return try await request(
            OperationInfo(service: "MyAssignments", operation: "GetMyAssignments", resourceType: "my_assignment", isMutation: false),
            method: "GET",
            path: "/my/assignments.json",
            retryConfig: Metadata.retryConfig(for: "GetMyAssignments")
        )
    }

    public func myCompletedAssignments() async throws -> [MyAssignment] {
        return try await request(
            OperationInfo(service: "MyAssignments", operation: "GetMyCompletedAssignments", resourceType: "my_completed_assignment", isMutation: false),
            method: "GET",
            path: "/my/assignments/completed.json",
            retryConfig: Metadata.retryConfig(for: "GetMyCompletedAssignments")
        )
    }

    public func myDueAssignments(options: MyDueAssignmentsMyAssignmentOptions? = nil) async throws -> [MyAssignment] {
        var queryItems: [URLQueryItem] = []
        if let scope = options?.scope {
            queryItems.append(URLQueryItem(name: "scope", value: scope))
        }
        return try await request(
            OperationInfo(service: "MyAssignments", operation: "GetMyDueAssignments", resourceType: "my_due_assignment", isMutation: false),
            method: "GET",
            path: "/my/assignments/due.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetMyDueAssignments")
        )
    }

    public func prioritizeAssignment(req: PrioritizeAssignmentRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "MyAssignments", operation: "PrioritizeAssignment", resourceType: "resource", isMutation: true),
            method: "POST",
            path: "/my/priorities.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "PrioritizeAssignment")
        )
    }

    public func reorderUpNext(req: ReorderUpNextRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "MyAssignments", operation: "ReorderUpNext", resourceType: "resource", isMutation: true),
            method: "POST",
            path: "/my/priority_moves.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "ReorderUpNext")
        )
    }
}
