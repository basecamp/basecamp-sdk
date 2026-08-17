// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetAssignedTodosResponseContent: Codable, Sendable {
    public var groupedBy: String?
    public var person: Person?
    public var todos: [Todo]?

    public init(groupedBy: String? = nil, person: Person? = nil, todos: [Todo]? = nil) {
        self.groupedBy = groupedBy
        self.person = person
        self.todos = todos
    }
}
