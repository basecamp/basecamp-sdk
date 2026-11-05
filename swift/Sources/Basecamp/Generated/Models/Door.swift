// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Door: Codable, Sendable {
    public let appUrl: String
    public let bucket: RecordingBucket
    public let createdAt: String
    public let creator: Person
    public let id: Int
    public let inheritsStatus: Bool
    public let service: DoorService
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var bookmarkUrl: String?
    public var description: String?
    public var position: Int32?
    public var subscriptionUrl: String?

    public init(
        appUrl: String,
        bucket: RecordingBucket,
        createdAt: String,
        creator: Person,
        id: Int,
        inheritsStatus: Bool,
        service: DoorService,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        bookmarkUrl: String? = nil,
        description: String? = nil,
        position: Int32? = nil,
        subscriptionUrl: String? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.service = service
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.bookmarkUrl = bookmarkUrl
        self.description = description
        self.position = position
        self.subscriptionUrl = subscriptionUrl
    }
}
