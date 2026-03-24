// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GaugeNeedle: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public let updatedAt: String
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var bucket: RecordingBucket?
    public var color: String?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var creator: Person?
    public var description: String?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var position: Int32?
    public var status: String?
    public var subscriptionUrl: String?
    public var title: String?
    public var type: String?
    public var url: String?
    public var visibleToClients: Bool?

    public init(
        createdAt: String,
        id: Int,
        updatedAt: String,
        appUrl: String? = nil,
        bookmarkUrl: String? = nil,
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        bucket: RecordingBucket? = nil,
        color: String? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        creator: Person? = nil,
        description: String? = nil,
        inheritsStatus: Bool? = nil,
        parent: RecordingParent? = nil,
        position: Int32? = nil,
        status: String? = nil,
        subscriptionUrl: String? = nil,
        title: String? = nil,
        type: String? = nil,
        url: String? = nil,
        visibleToClients: Bool? = nil
    ) {
        self.createdAt = createdAt
        self.id = id
        self.updatedAt = updatedAt
        self.appUrl = appUrl
        self.bookmarkUrl = bookmarkUrl
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.bucket = bucket
        self.color = color
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.creator = creator
        self.description = description
        self.inheritsStatus = inheritsStatus
        self.parent = parent
        self.position = position
        self.status = status
        self.subscriptionUrl = subscriptionUrl
        self.title = title
        self.type = type
        self.url = url
        self.visibleToClients = visibleToClients
    }
}
