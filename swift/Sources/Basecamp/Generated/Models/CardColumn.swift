// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CardColumn: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
    public let createdAt: String
    public let creator: Person
    public let id: Int
    public let inheritsStatus: Bool
    public let parent: RecordingParent
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var bookmarkUrl: String?
    public var cardsCount: Int32?
    public var cardsUrl: String?
    public var color: String?
    public var commentsCount: Int32?
    public var description: String?
    public var position: Int32?
    public var subscribers: [Person]?

    public init(
        appUrl: String,
        bucket: TodoBucket,
        createdAt: String,
        creator: Person,
        id: Int,
        inheritsStatus: Bool,
        parent: RecordingParent,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        bookmarkUrl: String? = nil,
        cardsCount: Int32? = nil,
        cardsUrl: String? = nil,
        color: String? = nil,
        commentsCount: Int32? = nil,
        description: String? = nil,
        position: Int32? = nil,
        subscribers: [Person]? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.parent = parent
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.bookmarkUrl = bookmarkUrl
        self.cardsCount = cardsCount
        self.cardsUrl = cardsUrl
        self.color = color
        self.commentsCount = commentsCount
        self.description = description
        self.position = position
        self.subscribers = subscribers
    }
}
