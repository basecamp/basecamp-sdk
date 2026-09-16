// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct InboxItem: Codable, Sendable {
    public let addressedAt: String
    public let addressingId: Int
    public let event: FeedEvent
    public let reason: String

    public init(
        addressedAt: String,
        addressingId: Int,
        event: FeedEvent,
        reason: String
    ) {
        self.addressedAt = addressedAt
        self.addressingId = addressingId
        self.event = event
        self.reason = reason
    }
}
