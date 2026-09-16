// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct PollEventsResponseContent: Codable, Sendable {
    public let events: [FeedEvent]
    public let position: String
    public var next: String?

    public init(events: [FeedEvent], position: String, next: String? = nil) {
        self.events = events
        self.position = position
        self.next = next
    }
}
