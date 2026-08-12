// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Tool: Codable, Sendable {
    public let createdAt: String
    public let creator: Person
    public let id: Int
    public let inheritsStatus: Bool
    public let title: String
    public let type: String
    public let updatedAt: String
    public let visibleToClients: Bool
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucket: RecordingBucket?
    public var enabled: Bool?
    public var name: String?
    public var parent: RecordingParent?
    public var position: Int32?
    public var status: String?
    public var subscriptionUrl: String?
    public var url: String?

    public init(
        createdAt: String,
        creator: Person,
        id: Int,
        inheritsStatus: Bool,
        title: String,
        type: String,
        updatedAt: String,
        visibleToClients: Bool,
        appUrl: String? = nil,
        bookmarkUrl: String? = nil,
        bucket: RecordingBucket? = nil,
        enabled: Bool? = nil,
        name: String? = nil,
        parent: RecordingParent? = nil,
        position: Int32? = nil,
        status: String? = nil,
        subscriptionUrl: String? = nil,
        url: String? = nil
    ) {
        self.createdAt = createdAt
        self.creator = creator
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.visibleToClients = visibleToClients
        self.appUrl = appUrl
        self.bookmarkUrl = bookmarkUrl
        self.bucket = bucket
        self.enabled = enabled
        self.name = name
        self.parent = parent
        self.position = position
        self.status = status
        self.subscriptionUrl = subscriptionUrl
        self.url = url
    }
}
