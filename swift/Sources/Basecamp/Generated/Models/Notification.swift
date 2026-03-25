// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Notification: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public let updatedAt: String
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucketName: String?
    public var contentExcerpt: String?
    public var creator: Person?
    public var imageUrl: String?
    public var memoryUrl: String?
    public var named: Bool?
    public var participants: [Person]?
    public var previewableAttachments: [PreviewableAttachment]?
    public var readAt: String?
    public var readableIdentifier: String?
    public var readableSgid: String?
    public var section: String?
    public var subscribed: Bool?
    public var subscriptionUrl: String?
    public var title: String?
    public var type: String?
    public var unreadAt: String?
    public var unreadCount: Int32?
    public var unreadUrl: String?

    public init(
        createdAt: String,
        id: Int,
        updatedAt: String,
        appUrl: String? = nil,
        bookmarkUrl: String? = nil,
        bucketName: String? = nil,
        contentExcerpt: String? = nil,
        creator: Person? = nil,
        imageUrl: String? = nil,
        memoryUrl: String? = nil,
        named: Bool? = nil,
        participants: [Person]? = nil,
        previewableAttachments: [PreviewableAttachment]? = nil,
        readAt: String? = nil,
        readableIdentifier: String? = nil,
        readableSgid: String? = nil,
        section: String? = nil,
        subscribed: Bool? = nil,
        subscriptionUrl: String? = nil,
        title: String? = nil,
        type: String? = nil,
        unreadAt: String? = nil,
        unreadCount: Int32? = nil,
        unreadUrl: String? = nil
    ) {
        self.createdAt = createdAt
        self.id = id
        self.updatedAt = updatedAt
        self.appUrl = appUrl
        self.bookmarkUrl = bookmarkUrl
        self.bucketName = bucketName
        self.contentExcerpt = contentExcerpt
        self.creator = creator
        self.imageUrl = imageUrl
        self.memoryUrl = memoryUrl
        self.named = named
        self.participants = participants
        self.previewableAttachments = previewableAttachments
        self.readAt = readAt
        self.readableIdentifier = readableIdentifier
        self.readableSgid = readableSgid
        self.section = section
        self.subscribed = subscribed
        self.subscriptionUrl = subscriptionUrl
        self.title = title
        self.type = type
        self.unreadAt = unreadAt
        self.unreadCount = unreadCount
        self.unreadUrl = unreadUrl
    }
}
