// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Gauge: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public let updatedAt: String
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucket: RecordingBucket?
    public var creator: Person?
    public var description: String?
    public var enabled: Bool?
    public var inheritsStatus: Bool?
    public var lastNeedleColor: String?
    public var lastNeedlePosition: Int32?
    public var previousNeedlePosition: Int32?
    public var status: String?
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
        bucket: RecordingBucket? = nil,
        creator: Person? = nil,
        description: String? = nil,
        enabled: Bool? = nil,
        inheritsStatus: Bool? = nil,
        lastNeedleColor: String? = nil,
        lastNeedlePosition: Int32? = nil,
        previousNeedlePosition: Int32? = nil,
        status: String? = nil,
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
        self.bucket = bucket
        self.creator = creator
        self.description = description
        self.enabled = enabled
        self.inheritsStatus = inheritsStatus
        self.lastNeedleColor = lastNeedleColor
        self.lastNeedlePosition = lastNeedlePosition
        self.previousNeedlePosition = previousNeedlePosition
        self.status = status
        self.title = title
        self.type = type
        self.url = url
        self.visibleToClients = visibleToClients
    }
}
