// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetMyNotificationsResponseContent: Codable, Sendable {
    public var bubbleUps: [Notification]?
    public var memories: [Notification]?
    public var reads: [Notification]?
    public var scheduledBubbleUps: [Notification]?
    public var unreads: [Notification]?
}
