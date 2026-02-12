// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct RepositionTodoRequest: Codable, Sendable {
    public var parentId: Int?
    public let position: Int32

    public init(parentId: Int? = nil, position: Int32) {
        self.parentId = parentId
        self.position = position
    }
}
