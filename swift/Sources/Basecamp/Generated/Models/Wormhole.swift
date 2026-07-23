// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Wormhole: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
    public let createdAt: String
    public let creator: Person
    public let destinationUrl: String?
    public let id: Int
    public let inheritsStatus: Bool
    public let linked: Bool
    public let parent: RecordingParent
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var bookmarkUrl: String?
    public var color: String?

    public init(
        appUrl: String,
        bucket: TodoBucket,
        createdAt: String,
        creator: Person,
        destinationUrl: String?,
        id: Int,
        inheritsStatus: Bool,
        linked: Bool,
        parent: RecordingParent,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        bookmarkUrl: String? = nil,
        color: String? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.destinationUrl = destinationUrl
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.linked = linked
        self.parent = parent
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.bookmarkUrl = bookmarkUrl
        self.color = color
    }

    enum CodingKeys: String, CodingKey {
        case appUrl
        case bucket
        case createdAt
        case creator
        case destinationUrl
        case id
        case inheritsStatus
        case linked
        case parent
        case status
        case title
        case type
        case updatedAt
        case url
        case visibleToClients
        case bookmarkUrl
        case color
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.bucket = try container.decode(TodoBucket.self, forKey: .bucket)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.creator = try container.decode(Person.self, forKey: .creator)
        self.destinationUrl = try container.decode(String?.self, forKey: .destinationUrl)
        self.id = try container.decode(Int.self, forKey: .id)
        self.inheritsStatus = try container.decode(Bool.self, forKey: .inheritsStatus)
        self.linked = try container.decode(Bool.self, forKey: .linked)
        self.parent = try container.decode(RecordingParent.self, forKey: .parent)
        self.status = try container.decode(String.self, forKey: .status)
        self.title = try container.decode(String.self, forKey: .title)
        self.type = try container.decode(String.self, forKey: .type)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
        self.url = try container.decode(String.self, forKey: .url)
        self.visibleToClients = try container.decode(Bool.self, forKey: .visibleToClients)
        self.bookmarkUrl = try container.decodeIfPresent(String.self, forKey: .bookmarkUrl)
        self.color = try container.decodeIfPresent(String.self, forKey: .color)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.bucket, forKey: .bucket)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.creator, forKey: .creator)
        try container.encode(self.destinationUrl, forKey: .destinationUrl)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.inheritsStatus, forKey: .inheritsStatus)
        try container.encode(self.linked, forKey: .linked)
        try container.encode(self.parent, forKey: .parent)
        try container.encode(self.status, forKey: .status)
        try container.encode(self.title, forKey: .title)
        try container.encode(self.type, forKey: .type)
        try container.encode(self.updatedAt, forKey: .updatedAt)
        try container.encode(self.url, forKey: .url)
        try container.encode(self.visibleToClients, forKey: .visibleToClients)
        try container.encodeIfPresent(self.bookmarkUrl, forKey: .bookmarkUrl)
        try container.encodeIfPresent(self.color, forKey: .color)
    }
}
