// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetAssignedTodosResponseContent: Codable, Sendable {
    public let groupedBy: String
    public let person: Person
    public let todos: [Todo]

    public init(groupedBy: String, person: Person, todos: [Todo]) {
        self.groupedBy = groupedBy
        self.person = person
        self.todos = todos
    }
}
