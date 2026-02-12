// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CardTable: Codable, Sendable {
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucket: TodoBucket?
    public var createdAt: String?
    public var creator: Person?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var lists: [CardColumn]?
    public var status: String?
    public var subscribers: [Person]?
    public var subscriptionUrl: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
