// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CardColumn: Codable, Sendable {
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucket: TodoBucket?
    public var cardsCount: Int32?
    public var cardsUrl: String?
    public var color: String?
    public var commentsCount: Int32?
    public var createdAt: String?
    public var creator: Person?
    public var description: String?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var position: Int32?
    public var status: String?
    public var subscribers: [Person]?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
