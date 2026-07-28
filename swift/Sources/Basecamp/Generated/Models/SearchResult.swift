// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct SearchResult: Codable, Sendable {
    public let appUrl: String
    public let content: String?
    public let description: String?
    public let id: Int
    public let title: String
    public let type: String
    public let url: String
    public var bookmarkUrl: String?
    public var bubbleUpUrl: String?
    public var bucket: RecordingBucket?
    public var contentAttachments: [RichTextAttachment]?
    public var createdAt: String?
    public var creator: Person?
    public var descriptionAttachments: [RichTextAttachment]?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var plainTextContent: String?
    public var plainTextDescription: String?
    public var status: String?
    public var subject: String?
    public var updatedAt: String?
    public var visibleToClients: Bool?

    public init(
        appUrl: String,
        content: String?,
        description: String?,
        id: Int,
        title: String,
        type: String,
        url: String,
        bookmarkUrl: String? = nil,
        bubbleUpUrl: String? = nil,
        bucket: RecordingBucket? = nil,
        contentAttachments: [RichTextAttachment]? = nil,
        createdAt: String? = nil,
        creator: Person? = nil,
        descriptionAttachments: [RichTextAttachment]? = nil,
        inheritsStatus: Bool? = nil,
        parent: RecordingParent? = nil,
        plainTextContent: String? = nil,
        plainTextDescription: String? = nil,
        status: String? = nil,
        subject: String? = nil,
        updatedAt: String? = nil,
        visibleToClients: Bool? = nil
    ) {
        self.appUrl = appUrl
        self.content = content
        self.description = description
        self.id = id
        self.title = title
        self.type = type
        self.url = url
        self.bookmarkUrl = bookmarkUrl
        self.bubbleUpUrl = bubbleUpUrl
        self.bucket = bucket
        self.contentAttachments = contentAttachments
        self.createdAt = createdAt
        self.creator = creator
        self.descriptionAttachments = descriptionAttachments
        self.inheritsStatus = inheritsStatus
        self.parent = parent
        self.plainTextContent = plainTextContent
        self.plainTextDescription = plainTextDescription
        self.status = status
        self.subject = subject
        self.updatedAt = updatedAt
        self.visibleToClients = visibleToClients
    }

    enum CodingKeys: String, CodingKey {
        case appUrl
        case content
        case description
        case id
        case title
        case type
        case url
        case bookmarkUrl
        case bubbleUpUrl
        case bucket
        case contentAttachments
        case createdAt
        case creator
        case descriptionAttachments
        case inheritsStatus
        case parent
        case plainTextContent
        case plainTextDescription
        case status
        case subject
        case updatedAt
        case visibleToClients
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.content = try container.decode(String?.self, forKey: .content)
        self.description = try container.decode(String?.self, forKey: .description)
        self.id = try container.decode(Int.self, forKey: .id)
        self.title = try container.decode(String.self, forKey: .title)
        self.type = try container.decode(String.self, forKey: .type)
        self.url = try container.decode(String.self, forKey: .url)
        self.bookmarkUrl = try container.decodeIfPresent(String.self, forKey: .bookmarkUrl)
        self.bubbleUpUrl = try container.decodeIfPresent(String.self, forKey: .bubbleUpUrl)
        self.bucket = try container.decodeIfPresent(RecordingBucket.self, forKey: .bucket)
        self.contentAttachments = try container.decodeIfPresent([RichTextAttachment].self, forKey: .contentAttachments)
        self.createdAt = try container.decodeIfPresent(String.self, forKey: .createdAt)
        self.creator = try container.decodeIfPresent(Person.self, forKey: .creator)
        self.descriptionAttachments = try container.decodeIfPresent([RichTextAttachment].self, forKey: .descriptionAttachments)
        self.inheritsStatus = try container.decodeIfPresent(Bool.self, forKey: .inheritsStatus)
        self.parent = try container.decodeIfPresent(RecordingParent.self, forKey: .parent)
        self.plainTextContent = try container.decodeIfPresent(String.self, forKey: .plainTextContent)
        self.plainTextDescription = try container.decodeIfPresent(String.self, forKey: .plainTextDescription)
        self.status = try container.decodeIfPresent(String.self, forKey: .status)
        self.subject = try container.decodeIfPresent(String.self, forKey: .subject)
        self.updatedAt = try container.decodeIfPresent(String.self, forKey: .updatedAt)
        self.visibleToClients = try container.decodeIfPresent(Bool.self, forKey: .visibleToClients)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.content, forKey: .content)
        try container.encode(self.description, forKey: .description)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.title, forKey: .title)
        try container.encode(self.type, forKey: .type)
        try container.encode(self.url, forKey: .url)
        try container.encodeIfPresent(self.bookmarkUrl, forKey: .bookmarkUrl)
        try container.encodeIfPresent(self.bubbleUpUrl, forKey: .bubbleUpUrl)
        try container.encodeIfPresent(self.bucket, forKey: .bucket)
        try container.encodeIfPresent(self.contentAttachments, forKey: .contentAttachments)
        try container.encodeIfPresent(self.createdAt, forKey: .createdAt)
        try container.encodeIfPresent(self.creator, forKey: .creator)
        try container.encodeIfPresent(self.descriptionAttachments, forKey: .descriptionAttachments)
        try container.encodeIfPresent(self.inheritsStatus, forKey: .inheritsStatus)
        try container.encodeIfPresent(self.parent, forKey: .parent)
        try container.encodeIfPresent(self.plainTextContent, forKey: .plainTextContent)
        try container.encodeIfPresent(self.plainTextDescription, forKey: .plainTextDescription)
        try container.encodeIfPresent(self.status, forKey: .status)
        try container.encodeIfPresent(self.subject, forKey: .subject)
        try container.encodeIfPresent(self.updatedAt, forKey: .updatedAt)
        try container.encodeIfPresent(self.visibleToClients, forKey: .visibleToClients)
    }
}
