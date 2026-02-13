// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Tool: Codable, Sendable {
    public let createdAt: String
    public let enabled: Bool
    public let id: Int
    public let name: String
    public let title: String
    public let updatedAt: String
    public var appUrl: String?
    public var bucket: RecordingBucket?
    public var position: Int32?
    public var status: String?
    public var url: String?

    public init(
        createdAt: String,
        enabled: Bool,
        id: Int,
        name: String,
        title: String,
        updatedAt: String,
        appUrl: String? = nil,
        bucket: RecordingBucket? = nil,
        position: Int32? = nil,
        status: String? = nil,
        url: String? = nil
    ) {
        self.createdAt = createdAt
        self.enabled = enabled
        self.id = id
        self.name = name
        self.title = title
        self.updatedAt = updatedAt
        self.appUrl = appUrl
        self.bucket = bucket
        self.position = position
        self.status = status
        self.url = url
    }
}
