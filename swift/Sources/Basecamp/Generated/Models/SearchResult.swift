// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct SearchResult: Codable, Sendable {
    public let appUrl: String
    public let id: Int
    public let title: String
    public let type: String
    public let url: String
    public var bookmarkUrl: String?
    public var bucket: RecordingBucket?
    public var content: String?
    public var createdAt: String?
    public var creator: Person?
    public var description: String?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var status: String?
    public var subject: String?
    public var updatedAt: String?
    public var visibleToClients: Bool?

    public init(
        appUrl: String,
        id: Int,
        title: String,
        type: String,
        url: String,
        bookmarkUrl: String? = nil,
        bucket: RecordingBucket? = nil,
        content: String? = nil,
        createdAt: String? = nil,
        creator: Person? = nil,
        description: String? = nil,
        inheritsStatus: Bool? = nil,
        parent: RecordingParent? = nil,
        status: String? = nil,
        subject: String? = nil,
        updatedAt: String? = nil,
        visibleToClients: Bool? = nil
    ) {
        self.appUrl = appUrl
        self.id = id
        self.title = title
        self.type = type
        self.url = url
        self.bookmarkUrl = bookmarkUrl
        self.bucket = bucket
        self.content = content
        self.createdAt = createdAt
        self.creator = creator
        self.description = description
        self.inheritsStatus = inheritsStatus
        self.parent = parent
        self.status = status
        self.subject = subject
        self.updatedAt = updatedAt
        self.visibleToClients = visibleToClients
    }
}
