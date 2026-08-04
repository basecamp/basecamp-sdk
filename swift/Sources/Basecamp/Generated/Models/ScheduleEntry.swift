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
}
