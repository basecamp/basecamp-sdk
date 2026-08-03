// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Folder: Codable, Sendable {
    public let bucketIds: [Int]
    public let color: String?
    public let createdAt: String
    public let gaugesUrl: String?
    public let id: Int
    public let imageUrl: String?
    public let isEmojiOnlyName: Bool
    public let name: String
    public let starUrl: String
    public let type: String
    public let updatedAt: String
    public let url: String

    public init(
        bucketIds: [Int],
        color: String?,
        createdAt: String,
        gaugesUrl: String?,
        id: Int,
        imageUrl: String?,
        isEmojiOnlyName: Bool,
        name: String,
        starUrl: String,
        type: String,
        updatedAt: String,
        url: String
    ) {
        self.bucketIds = bucketIds
        self.color = color
        self.createdAt = createdAt
        self.gaugesUrl = gaugesUrl
        self.id = id
        self.imageUrl = imageUrl
        self.isEmojiOnlyName = isEmojiOnlyName
        self.name = name
        self.starUrl = starUrl
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
    }

    enum CodingKeys: String, CodingKey {
        case bucketIds
        case color
        case createdAt
        case gaugesUrl
        case id
        case imageUrl
        case isEmojiOnlyName
        case name
        case starUrl
        case type
        case updatedAt
        case url
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.bucketIds = try container.decode([Int].self, forKey: .bucketIds)
        self.color = try container.decode(String?.self, forKey: .color)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.gaugesUrl = try container.decode(String?.self, forKey: .gaugesUrl)
        self.id = try container.decode(Int.self, forKey: .id)
        self.imageUrl = try container.decode(String?.self, forKey: .imageUrl)
        self.isEmojiOnlyName = try container.decode(Bool.self, forKey: .isEmojiOnlyName)
        self.name = try container.decode(String.self, forKey: .name)
        self.starUrl = try container.decode(String.self, forKey: .starUrl)
        self.type = try container.decode(String.self, forKey: .type)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
        self.url = try container.decode(String.self, forKey: .url)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.bucketIds, forKey: .bucketIds)
        try container.encode(self.color, forKey: .color)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.gaugesUrl, forKey: .gaugesUrl)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.imageUrl, forKey: .imageUrl)
        try container.encode(self.isEmojiOnlyName, forKey: .isEmojiOnlyName)
        try container.encode(self.name, forKey: .name)
        try container.encode(self.starUrl, forKey: .starUrl)
        try container.encode(self.type, forKey: .type)
        try container.encode(self.updatedAt, forKey: .updatedAt)
        try container.encode(self.url, forKey: .url)
    }
}
