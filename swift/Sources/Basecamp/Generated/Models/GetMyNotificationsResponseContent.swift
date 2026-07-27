// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetMyNotificationsResponseContent: Codable, Sendable {
    public let bubbleUpsCount: Int32
    public let scheduledBubbleUpsCount: Int32
    public var bubbleUps: [Notification]?
    public var memories: [Notification]?
    public var reads: [Notification]?
    public var scheduledBubbleUps: [Notification]?
    public var unreads: [Notification]?

    public init(
        bubbleUpsCount: Int32,
        scheduledBubbleUpsCount: Int32,
        bubbleUps: [Notification]? = nil,
        memories: [Notification]? = nil,
        reads: [Notification]? = nil,
        scheduledBubbleUps: [Notification]? = nil,
        unreads: [Notification]? = nil
    ) {
        self.bubbleUpsCount = bubbleUpsCount
        self.scheduledBubbleUpsCount = scheduledBubbleUpsCount
        self.bubbleUps = bubbleUps
        self.memories = memories
        self.reads = reads
        self.scheduledBubbleUps = scheduledBubbleUps
        self.unreads = unreads
    }
}
