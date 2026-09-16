// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ScheduleEntry: Codable, Sendable {
    public let allDay: Bool
    public let appUrl: String
    public let bucket: TodoBucket
    public let createdAt: String
    public let creator: Person
    public let descriptionAttachments: [RichTextAttachment]
    public let endsAt: String
    public let id: Int
    public let inheritsStatus: Bool
    public let parent: RecordingParent
    public let startsAt: String
    public let status: String
    public let summary: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var description: String?
    public var highlighted: Bool?
    public var joinUrl: String?
    public var participants: [Person]?
    public var subscriptionUrl: String?

    public init(
        allDay: Bool,
        appUrl: String,
        bucket: TodoBucket,
        createdAt: String,
        creator: Person,
        descriptionAttachments: [RichTextAttachment],
        endsAt: String,
        id: Int,
        inheritsStatus: Bool,
        parent: RecordingParent,
        startsAt: String,
        status: String,
        summary: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        bookmarkUrl: String? = nil,
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        description: String? = nil,
        highlighted: Bool? = nil,
        joinUrl: String? = nil,
        participants: [Person]? = nil,
        subscriptionUrl: String? = nil
    ) {
        self.allDay = allDay
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.descriptionAttachments = descriptionAttachments
        self.endsAt = endsAt
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.parent = parent
        self.startsAt = startsAt
        self.status = status
        self.summary = summary
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.bookmarkUrl = bookmarkUrl
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.description = description
        self.highlighted = highlighted
        self.joinUrl = joinUrl
        self.participants = participants
        self.subscriptionUrl = subscriptionUrl
    }

    enum CodingKeys: String, CodingKey {
        case allDay
        case appUrl
        case bucket
        case createdAt
        case creator
        case descriptionAttachments
        case endsAt
        case id
        case inheritsStatus
        case parent
        case startsAt
        case status
        case summary
        case title
        case type
        case updatedAt
        case url
        case visibleToClients
        case bookmarkUrl
        case boostsCount
        case boostsUrl
        case commentsCount
        case commentsUrl
        case description
        case highlighted
        case joinUrl
        case participants
        case subscriptionUrl
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.allDay = try container.decode(Bool.self, forKey: .allDay)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.bucket = try container.decode(TodoBucket.self, forKey: .bucket)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.creator = try container.decode(Person.self, forKey: .creator)
        self.descriptionAttachments = try container.decode([RichTextAttachment].self, forKey: .descriptionAttachments)
        self.endsAt = try container.decode(String.self, forKey: .endsAt)
        self.id = try container.decode(Int.self, forKey: .id)
        self.inheritsStatus = try container.decode(Bool.self, forKey: .inheritsStatus)
        self.parent = try container.decode(RecordingParent.self, forKey: .parent)
        self.startsAt = try container.decode(String.self, forKey: .startsAt)
        self.status = try container.decode(String.self, forKey: .status)
        self.summary = try container.decode(String.self, forKey: .summary)
        self.title = try container.decode(String.self, forKey: .title)
        self.type = try container.decode(String.self, forKey: .type)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
        self.url = try container.decode(String.self, forKey: .url)
        self.visibleToClients = try container.decode(Bool.self, forKey: .visibleToClients)
        self.bookmarkUrl = try container.decodeIfPresent(String.self, forKey: .bookmarkUrl)
        self.boostsCount = try container.decodeIfPresent(Int32.self, forKey: .boostsCount)
        self.boostsUrl = try container.decodeIfPresent(String.self, forKey: .boostsUrl)
        self.commentsCount = try container.decodeIfPresent(Int32.self, forKey: .commentsCount)
        self.commentsUrl = try container.decodeIfPresent(String.self, forKey: .commentsUrl)
        self.description = try container.decodeIfPresent(String.self, forKey: .description)
        self.highlighted = try container.decodeIfPresent(Bool.self, forKey: .highlighted)
        self.joinUrl = try container.decodeIfPresent(String.self, forKey: .joinUrl)
        self.participants = try container.decodePeopleIfPresent([Person].self, forKey: .participants)
        self.subscriptionUrl = try container.decodeIfPresent(String.self, forKey: .subscriptionUrl)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.allDay, forKey: .allDay)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.bucket, forKey: .bucket)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.creator, forKey: .creator)
        try container.encode(self.descriptionAttachments, forKey: .descriptionAttachments)
        try container.encode(self.endsAt, forKey: .endsAt)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.inheritsStatus, forKey: .inheritsStatus)
        try container.encode(self.parent, forKey: .parent)
        try container.encode(self.startsAt, forKey: .startsAt)
        try container.encode(self.status, forKey: .status)
        try container.encode(self.summary, forKey: .summary)
        try container.encode(self.title, forKey: .title)
        try container.encode(self.type, forKey: .type)
        try container.encode(self.updatedAt, forKey: .updatedAt)
        try container.encode(self.url, forKey: .url)
        try container.encode(self.visibleToClients, forKey: .visibleToClients)
        try container.encodeIfPresent(self.bookmarkUrl, forKey: .bookmarkUrl)
        try container.encodeIfPresent(self.boostsCount, forKey: .boostsCount)
        try container.encodeIfPresent(self.boostsUrl, forKey: .boostsUrl)
        try container.encodeIfPresent(self.commentsCount, forKey: .commentsCount)
        try container.encodeIfPresent(self.commentsUrl, forKey: .commentsUrl)
        try container.encodeIfPresent(self.description, forKey: .description)
        try container.encodeIfPresent(self.highlighted, forKey: .highlighted)
        try container.encodeIfPresent(self.joinUrl, forKey: .joinUrl)
        try container.encodeIfPresent(self.participants, forKey: .participants)
        try container.encodeIfPresent(self.subscriptionUrl, forKey: .subscriptionUrl)
    }
}
