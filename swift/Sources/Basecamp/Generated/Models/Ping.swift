// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Ping: Codable, Sendable {
    public let appUrl: String
    public let avatarUrl: String
    public let bookmarkUrl: String
    public let createdAt: String
    public let id: Int
    public let links: PingLinks
    public let named: Bool
    public let people: [Person]
    public let roomUrl: String
    public let title: String
    public let updatedAt: String
    public let url: String
    public var editTitleUrl: String?
    public var imageUrl: String?

    public init(
        appUrl: String,
        avatarUrl: String,
        bookmarkUrl: String,
        createdAt: String,
        id: Int,
        links: PingLinks,
        named: Bool,
        people: [Person],
        roomUrl: String,
        title: String,
        updatedAt: String,
        url: String,
        editTitleUrl: String? = nil,
        imageUrl: String? = nil
    ) {
        self.appUrl = appUrl
        self.avatarUrl = avatarUrl
        self.bookmarkUrl = bookmarkUrl
        self.createdAt = createdAt
        self.id = id
        self.links = links
        self.named = named
        self.people = people
        self.roomUrl = roomUrl
        self.title = title
        self.updatedAt = updatedAt
        self.url = url
        self.editTitleUrl = editTitleUrl
        self.imageUrl = imageUrl
    }

    enum CodingKeys: String, CodingKey {
        case appUrl
        case avatarUrl
        case bookmarkUrl
        case createdAt
        case id
        case links
        case named
        case people
        case roomUrl
        case title
        case updatedAt
        case url
        case editTitleUrl
        case imageUrl
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.avatarUrl = try container.decode(String.self, forKey: .avatarUrl)
        self.bookmarkUrl = try container.decode(String.self, forKey: .bookmarkUrl)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.id = try container.decode(Int.self, forKey: .id)
        self.links = try container.decode(PingLinks.self, forKey: .links)
        self.named = try container.decode(Bool.self, forKey: .named)
        self.people = try container.decodePeople([Person].self, forKey: .people)
        self.roomUrl = try container.decode(String.self, forKey: .roomUrl)
        self.title = try container.decode(String.self, forKey: .title)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
        self.url = try container.decode(String.self, forKey: .url)
        self.editTitleUrl = try container.decodeIfPresent(String.self, forKey: .editTitleUrl)
        self.imageUrl = try container.decodeIfPresent(String.self, forKey: .imageUrl)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.avatarUrl, forKey: .avatarUrl)
        try container.encode(self.bookmarkUrl, forKey: .bookmarkUrl)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.links, forKey: .links)
        try container.encode(self.named, forKey: .named)
        try container.encode(self.people, forKey: .people)
        try container.encode(self.roomUrl, forKey: .roomUrl)
        try container.encode(self.title, forKey: .title)
        try container.encode(self.updatedAt, forKey: .updatedAt)
        try container.encode(self.url, forKey: .url)
        try container.encodeIfPresent(self.editTitleUrl, forKey: .editTitleUrl)
        try container.encodeIfPresent(self.imageUrl, forKey: .imageUrl)
    }
}
