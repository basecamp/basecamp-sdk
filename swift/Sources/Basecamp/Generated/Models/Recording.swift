// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Recording: Codable, Sendable {
    public let appUrl: String
    public let bucket: RecordingBucket
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
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var content: String?
    public var subscriptionUrl: String?

    public init(
        appUrl: String,
        bucket: RecordingBucket,
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
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        content: String? = nil,
        subscriptionUrl: String? = nil
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
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.content = content
        self.subscriptionUrl = subscriptionUrl
    }
}
