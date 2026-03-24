// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyDueAssignmentsMyAssignmentOptions: Sendable {
    public var scope: String?

    public init(scope: String? = nil) {
        self.scope = scope
    }
}


public final class MyAssignmentsService: BaseService, @unchecked Sendable {
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
}
