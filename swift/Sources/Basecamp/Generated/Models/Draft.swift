// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Draft: Codable, Sendable {
    public let appUrl: String
    public let bucket: DraftBucket
    public let createdAt: String
    public let excerpt: String
    public let id: Int
    public let parent: DraftParent?
    public let scheduledPostingAt: String?
    public let title: String
    public let type: String
    public let updatedAt: String

    public init(
        appUrl: String,
        bucket: DraftBucket,
        createdAt: String,
        excerpt: String,
        id: Int,
        parent: DraftParent?,
        scheduledPostingAt: String?,
        title: String,
        type: String,
        updatedAt: String
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.excerpt = excerpt
        self.id = id
        self.parent = parent
        self.scheduledPostingAt = scheduledPostingAt
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
    }

    enum CodingKeys: String, CodingKey {
        case appUrl
        case bucket
        case createdAt
        case excerpt
        case id
        case parent
        case scheduledPostingAt
        case title
        case type
        case updatedAt
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.bucket = try container.decode(DraftBucket.self, forKey: .bucket)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.excerpt = try container.decode(String.self, forKey: .excerpt)
        self.id = try container.decode(Int.self, forKey: .id)
        self.parent = try container.decode(DraftParent?.self, forKey: .parent)
        self.scheduledPostingAt = try container.decode(String?.self, forKey: .scheduledPostingAt)
        self.title = try container.decode(String.self, forKey: .title)
        self.type = try container.decode(String.self, forKey: .type)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.bucket, forKey: .bucket)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.excerpt, forKey: .excerpt)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.parent, forKey: .parent)
        try container.encode(self.scheduledPostingAt, forKey: .scheduledPostingAt)
        try container.encode(self.title, forKey: .title)
        try container.encode(self.type, forKey: .type)
        try container.encode(self.updatedAt, forKey: .updatedAt)
    }
}
