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
    public var onHold: CardColumnOnHold?
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
        onHold: CardColumnOnHold? = nil,
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
        self.onHold = onHold
        self.position = position
        self.subscribers = subscribers
    }

    enum CodingKeys: String, CodingKey {
        case appUrl
        case bucket
        case createdAt
        case creator
        case id
        case inheritsStatus
        case parent
        case status
        case title
        case type
        case updatedAt
        case url
        case visibleToClients
        case bookmarkUrl
        case cardsCount
        case cardsUrl
        case color
        case commentsCount
        case description
        case onHold
        case position
        case subscribers
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.bucket = try container.decode(TodoBucket.self, forKey: .bucket)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.creator = try container.decode(Person.self, forKey: .creator)
        self.id = try container.decode(Int.self, forKey: .id)
        self.inheritsStatus = try container.decode(Bool.self, forKey: .inheritsStatus)
        self.parent = try container.decode(RecordingParent.self, forKey: .parent)
        self.status = try container.decode(String.self, forKey: .status)
        self.title = try container.decode(String.self, forKey: .title)
        self.type = try container.decode(String.self, forKey: .type)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
        self.url = try container.decode(String.self, forKey: .url)
        self.visibleToClients = try container.decode(Bool.self, forKey: .visibleToClients)
        self.bookmarkUrl = try container.decodeIfPresent(String.self, forKey: .bookmarkUrl)
        self.cardsCount = try container.decodeIfPresent(Int32.self, forKey: .cardsCount)
        self.cardsUrl = try container.decodeIfPresent(String.self, forKey: .cardsUrl)
        self.color = try container.decodeIfPresent(String.self, forKey: .color)
        self.commentsCount = try container.decodeIfPresent(Int32.self, forKey: .commentsCount)
        self.description = try container.decodeIfPresent(String.self, forKey: .description)
        self.onHold = try container.decodeIfPresent(CardColumnOnHold.self, forKey: .onHold)
        self.position = try container.decodeIfPresent(Int32.self, forKey: .position)
        self.subscribers = try container.decodePeopleIfPresent([Person].self, forKey: .subscribers)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.bucket, forKey: .bucket)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.creator, forKey: .creator)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.inheritsStatus, forKey: .inheritsStatus)
        try container.encode(self.parent, forKey: .parent)
        try container.encode(self.status, forKey: .status)
        try container.encode(self.title, forKey: .title)
        try container.encode(self.type, forKey: .type)
        try container.encode(self.updatedAt, forKey: .updatedAt)
        try container.encode(self.url, forKey: .url)
        try container.encode(self.visibleToClients, forKey: .visibleToClients)
        try container.encodeIfPresent(self.bookmarkUrl, forKey: .bookmarkUrl)
        try container.encodeIfPresent(self.cardsCount, forKey: .cardsCount)
        try container.encodeIfPresent(self.cardsUrl, forKey: .cardsUrl)
        try container.encodeIfPresent(self.color, forKey: .color)
        try container.encodeIfPresent(self.commentsCount, forKey: .commentsCount)
        try container.encodeIfPresent(self.description, forKey: .description)
        try container.encodeIfPresent(self.onHold, forKey: .onHold)
        try container.encodeIfPresent(self.position, forKey: .position)
        try container.encodeIfPresent(self.subscribers, forKey: .subscribers)
    }
}
