// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetMyAssignmentsResponseContent: Codable, Sendable {
    public var nonPriorities: [MyAssignment]?
    public var priorities: [MyAssignment]?

    public init(nonPriorities: [MyAssignment]? = nil, priorities: [MyAssignment]? = nil) {
        self.nonPriorities = nonPriorities
        self.priorities = priorities
    }
}
