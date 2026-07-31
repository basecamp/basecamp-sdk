// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Calendar: Codable, Sendable {
    public let appUrl: String
    public let color: String
    public let createdAt: String
    public let id: Int
    public let name: String
    public let scheduleUrl: String
    public let type: String
    public let updatedAt: String
    public let url: String

    public init(
        appUrl: String,
        color: String,
        createdAt: String,
        id: Int,
        name: String,
        scheduleUrl: String,
        type: String,
        updatedAt: String,
        url: String
    ) {
        self.appUrl = appUrl
        self.color = color
        self.createdAt = createdAt
        self.id = id
        self.name = name
        self.scheduleUrl = scheduleUrl
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
    }
}
