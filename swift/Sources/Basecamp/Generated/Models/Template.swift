// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Template: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public let name: String
    public let updatedAt: String
    public var appUrl: String?
    public var description: String?
    public var dock: [DockItem]?
    public var status: String?
    public var url: String?

    public init(
        createdAt: String,
        id: Int,
        name: String,
        updatedAt: String,
        appUrl: String? = nil,
        description: String? = nil,
        dock: [DockItem]? = nil,
        status: String? = nil,
        url: String? = nil
    ) {
        self.createdAt = createdAt
        self.id = id
        self.name = name
        self.updatedAt = updatedAt
        self.appUrl = appUrl
        self.description = description
        self.dock = dock
        self.status = status
        self.url = url
    }
}
