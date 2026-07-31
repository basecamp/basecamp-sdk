// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyNote: Codable, Sendable {
    public let appUrl: String
    public let content: String
    public let contentAttachments: [RichTextAttachment]
    public let createdAt: String?
    public let id: Int?
    public let type: String
    public let updatedAt: String?
    public let url: String

    public init(
        appUrl: String,
        content: String,
        contentAttachments: [RichTextAttachment],
        createdAt: String?,
        id: Int?,
        type: String,
        updatedAt: String?,
        url: String
    ) {
        self.appUrl = appUrl
        self.content = content
        self.contentAttachments = contentAttachments
        self.createdAt = createdAt
        self.id = id
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
    }

    enum CodingKeys: String, CodingKey {
        case appUrl
        case content
        case contentAttachments
        case createdAt
        case id
        case type
        case updatedAt
        case url
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.content = try container.decode(String.self, forKey: .content)
        self.contentAttachments = try container.decode([RichTextAttachment].self, forKey: .contentAttachments)
        self.createdAt = try container.decode(String?.self, forKey: .createdAt)
        self.id = try container.decode(Int?.self, forKey: .id)
        self.type = try container.decode(String.self, forKey: .type)
        self.updatedAt = try container.decode(String?.self, forKey: .updatedAt)
        self.url = try container.decode(String.self, forKey: .url)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.content, forKey: .content)
        try container.encode(self.contentAttachments, forKey: .contentAttachments)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.type, forKey: .type)
        try container.encode(self.updatedAt, forKey: .updatedAt)
        try container.encode(self.url, forKey: .url)
    }
}
