// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct FeedEvent: Codable, Sendable {
    public let action: String
    public let bucketId: Int
    public let createdAt: String
    public let creatorId: Int
    public let eventType: String
    public let id: Int
    public let kind: String
    public let recordingId: Int
    public var details: FeedEventDetails?
    public var performedById: Int?

    public init(
        action: String,
        bucketId: Int,
        createdAt: String,
        creatorId: Int,
        eventType: String,
        id: Int,
        kind: String,
        recordingId: Int,
        details: FeedEventDetails? = nil,
        performedById: Int? = nil
    ) {
        self.action = action
        self.bucketId = bucketId
        self.createdAt = createdAt
        self.creatorId = creatorId
        self.eventType = eventType
        self.id = id
        self.kind = kind
        self.recordingId = recordingId
        self.details = details
        self.performedById = performedById
    }
}
