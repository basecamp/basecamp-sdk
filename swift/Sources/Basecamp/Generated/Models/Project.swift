// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Project: Codable, Sendable {
    public let appUrl: String
    public let createdAt: String
    public let id: Int
    public let name: String
    public let status: String
    public let updatedAt: String
    public let url: String
    public var bookmarkUrl: String?
    public var bookmarked: Bool?
    public var clientCompany: ClientCompany?
    public var clientsEnabled: Bool?
    public var clientside: ClientSide?
    public var description: String?
    public var dock: [DockItem]?
    public var purpose: String?

    public init(
        appUrl: String,
        createdAt: String,
        id: Int,
        name: String,
        status: String,
        updatedAt: String,
        url: String,
        bookmarkUrl: String? = nil,
        bookmarked: Bool? = nil,
        clientCompany: ClientCompany? = nil,
        clientsEnabled: Bool? = nil,
        clientside: ClientSide? = nil,
        description: String? = nil,
        dock: [DockItem]? = nil,
        purpose: String? = nil
    ) {
        self.appUrl = appUrl
        self.createdAt = createdAt
        self.id = id
        self.name = name
        self.status = status
        self.updatedAt = updatedAt
        self.url = url
        self.bookmarkUrl = bookmarkUrl
        self.bookmarked = bookmarked
        self.clientCompany = clientCompany
        self.clientsEnabled = clientsEnabled
        self.clientside = clientside
        self.description = description
        self.dock = dock
        self.purpose = purpose
    }
}
