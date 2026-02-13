// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct DockItem: Codable, Sendable {
    public let appUrl: String
    public let enabled: Bool
    public let id: Int
    public let name: String
    public let title: String
    public let url: String
    public var position: Int32?

    public init(
        appUrl: String,
        enabled: Bool,
        id: Int,
        name: String,
        title: String,
        url: String,
        position: Int32? = nil
    ) {
        self.appUrl = appUrl
        self.enabled = enabled
        self.id = id
        self.name = name
        self.title = title
        self.url = url
        self.position = position
    }
}
