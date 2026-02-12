// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Project: Codable, Sendable {
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bookmarked: Bool?
    public var clientCompany: ClientCompany?
    public var clientsEnabled: Bool?
    public var clientside: ClientSide?
    public var createdAt: String?
    public var description: String?
    public var dock: [DockItem]?
    public var id: Int?
    public var name: String?
    public var purpose: String?
    public var status: String?
    public var updatedAt: String?
    public var url: String?
}
