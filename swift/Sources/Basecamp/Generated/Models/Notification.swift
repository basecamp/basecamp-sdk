// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Notification: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public let updatedAt: String
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bubbleUpAt: String?
    public var bubbleUpUrl: String?
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
        bubbleUpAt: String? = nil,
        bubbleUpUrl: String? = nil,
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
        self.bubbleUpAt = bubbleUpAt
        self.bubbleUpUrl = bubbleUpUrl
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

    enum CodingKeys: String, CodingKey {
        case createdAt
        case id
        case updatedAt
        case appUrl
        case bookmarkUrl
        case bubbleUpAt
        case bubbleUpUrl
        case bucketName
        case contentExcerpt
        case creator
        case imageUrl
        case memoryUrl
        case named
        case participants
        case previewableAttachments
        case readAt
        case readableIdentifier
        case readableSgid
        case section
        case subscribed
        case subscriptionUrl
        case title
        case type
        case unreadAt
        case unreadCount
        case unreadUrl
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.id = try container.decode(Int.self, forKey: .id)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
        self.appUrl = try container.decodeIfPresent(String.self, forKey: .appUrl)
        self.bookmarkUrl = try container.decodeIfPresent(String.self, forKey: .bookmarkUrl)
        self.bubbleUpAt = try container.decodeIfPresent(String.self, forKey: .bubbleUpAt)
        self.bubbleUpUrl = try container.decodeIfPresent(String.self, forKey: .bubbleUpUrl)
        self.bucketName = try container.decodeIfPresent(String.self, forKey: .bucketName)
        self.contentExcerpt = try container.decodeIfPresent(String.self, forKey: .contentExcerpt)
        self.creator = try container.decodeIfPresent(Person.self, forKey: .creator)
        self.imageUrl = try container.decodeIfPresent(String.self, forKey: .imageUrl)
        self.memoryUrl = try container.decodeIfPresent(String.self, forKey: .memoryUrl)
        self.named = try container.decodeIfPresent(Bool.self, forKey: .named)
        self.participants = try container.decodePeopleIfPresent([Person].self, forKey: .participants)
        self.previewableAttachments = try container.decodeIfPresent([PreviewableAttachment].self, forKey: .previewableAttachments)
        self.readAt = try container.decodeIfPresent(String.self, forKey: .readAt)
        self.readableIdentifier = try container.decodeIfPresent(String.self, forKey: .readableIdentifier)
        self.readableSgid = try container.decodeIfPresent(String.self, forKey: .readableSgid)
        self.section = try container.decodeIfPresent(String.self, forKey: .section)
        self.subscribed = try container.decodeIfPresent(Bool.self, forKey: .subscribed)
        self.subscriptionUrl = try container.decodeIfPresent(String.self, forKey: .subscriptionUrl)
        self.title = try container.decodeIfPresent(String.self, forKey: .title)
        self.type = try container.decodeIfPresent(String.self, forKey: .type)
        self.unreadAt = try container.decodeIfPresent(String.self, forKey: .unreadAt)
        self.unreadCount = try container.decodeIfPresent(Int32.self, forKey: .unreadCount)
        self.unreadUrl = try container.decodeIfPresent(String.self, forKey: .unreadUrl)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.updatedAt, forKey: .updatedAt)
        try container.encodeIfPresent(self.appUrl, forKey: .appUrl)
        try container.encodeIfPresent(self.bookmarkUrl, forKey: .bookmarkUrl)
        try container.encodeIfPresent(self.bubbleUpAt, forKey: .bubbleUpAt)
        try container.encodeIfPresent(self.bubbleUpUrl, forKey: .bubbleUpUrl)
        try container.encodeIfPresent(self.bucketName, forKey: .bucketName)
        try container.encodeIfPresent(self.contentExcerpt, forKey: .contentExcerpt)
        try container.encodeIfPresent(self.creator, forKey: .creator)
        try container.encodeIfPresent(self.imageUrl, forKey: .imageUrl)
        try container.encodeIfPresent(self.memoryUrl, forKey: .memoryUrl)
        try container.encodeIfPresent(self.named, forKey: .named)
        try container.encodeIfPresent(self.participants, forKey: .participants)
        try container.encodeIfPresent(self.previewableAttachments, forKey: .previewableAttachments)
        try container.encodeIfPresent(self.readAt, forKey: .readAt)
        try container.encodeIfPresent(self.readableIdentifier, forKey: .readableIdentifier)
        try container.encodeIfPresent(self.readableSgid, forKey: .readableSgid)
        try container.encodeIfPresent(self.section, forKey: .section)
        try container.encodeIfPresent(self.subscribed, forKey: .subscribed)
        try container.encodeIfPresent(self.subscriptionUrl, forKey: .subscriptionUrl)
        try container.encodeIfPresent(self.title, forKey: .title)
        try container.encodeIfPresent(self.type, forKey: .type)
        try container.encodeIfPresent(self.unreadAt, forKey: .unreadAt)
        try container.encodeIfPresent(self.unreadCount, forKey: .unreadCount)
        try container.encodeIfPresent(self.unreadUrl, forKey: .unreadUrl)
    }
}
