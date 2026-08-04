// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpcomingSchedulePerson: Codable, Sendable {
    public let avatarUrl: String
    public let id: Int
    public let name: String

    public init(avatarUrl: String, id: Int, name: String) {
        self.avatarUrl = avatarUrl
        self.id = id
        self.name = name
    }
}
