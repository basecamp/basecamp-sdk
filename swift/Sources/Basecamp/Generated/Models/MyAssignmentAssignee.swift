// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignmentAssignee: Codable, Sendable {
    public let id: Int
    public var avatarUrl: String?
    public var name: String?

    public init(id: Int, avatarUrl: String? = nil, name: String? = nil) {
        self.id = id
        self.avatarUrl = avatarUrl
        self.name = name
    }
}
