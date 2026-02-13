// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MessageType: Codable, Sendable {
    public let createdAt: String
    public let icon: String
    public let id: Int
    public let name: String
    public let updatedAt: String

    public init(
        createdAt: String,
        icon: String,
        id: Int,
        name: String,
        updatedAt: String
    ) {
        self.createdAt = createdAt
        self.icon = icon
        self.id = id
        self.name = name
        self.updatedAt = updatedAt
    }
}
