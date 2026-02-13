// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Upload: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
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
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var byteSize: Int?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var contentType: String?
    public var description: String?
    public var downloadUrl: String?
    public var filename: String?
    public var height: Int32?
    public var position: Int32?
    public var subscriptionUrl: String?
    public var width: Int32?

    public init(
        appUrl: String,
        bucket: TodoBucket,
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
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        byteSize: Int? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        contentType: String? = nil,
        description: String? = nil,
        downloadUrl: String? = nil,
        filename: String? = nil,
        height: Int32? = nil,
        position: Int32? = nil,
        subscriptionUrl: String? = nil,
        width: Int32? = nil
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
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.byteSize = byteSize
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.contentType = contentType
        self.description = description
        self.downloadUrl = downloadUrl
        self.filename = filename
        self.height = height
        self.position = position
        self.subscriptionUrl = subscriptionUrl
        self.width = width
    }
}
