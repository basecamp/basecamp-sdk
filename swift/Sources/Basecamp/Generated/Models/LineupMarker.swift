// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct LineupMarker: Codable, Sendable {
    public let createdAt: String
    public let date: String
    public let id: Int
    public let name: String
    public let updatedAt: String

    public init(
        createdAt: String,
        date: String,
        id: Int,
        name: String,
        updatedAt: String
    ) {
        self.createdAt = createdAt
        self.date = date
        self.id = id
        self.name = name
        self.updatedAt = updatedAt
    }
}
