// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct HillChartVersion: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
    public let createdAt: String
    public let creator: Person
    public let descriptionAttachments: [RichTextAttachment]
    public let dots: [HillChartVersionDot]
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
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var description: String?
    public var subscriptionUrl: String?

    public init(
        appUrl: String,
        bucket: TodoBucket,
        createdAt: String,
        creator: Person,
        descriptionAttachments: [RichTextAttachment],
        dots: [HillChartVersionDot],
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
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        description: String? = nil,
        subscriptionUrl: String? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.descriptionAttachments = descriptionAttachments
        self.dots = dots
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
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.description = description
        self.subscriptionUrl = subscriptionUrl
    }
}
