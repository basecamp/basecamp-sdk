// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignmentPerson: Codable, Sendable {
    public let id: Int
    public let name: String
    public var avatarUrl: String?

    public init(id: Int, name: String, avatarUrl: String? = nil) {
        self.id = id
        self.name = name
        self.avatarUrl = avatarUrl
    }
}
