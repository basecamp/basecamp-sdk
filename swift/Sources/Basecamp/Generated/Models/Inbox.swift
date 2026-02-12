// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Inbox: Codable, Sendable {
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucket: TodoBucket?
    public var createdAt: String?
    public var creator: Person?
    public var forwardsCount: Int32?
    public var forwardsUrl: String?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var position: Int32?
    public var status: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
