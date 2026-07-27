// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct TimelineEventData: Codable, Sendable {
    public let allDay: Bool
    public let endsAt: String
    public let startsAt: String

    public init(allDay: Bool, endsAt: String, startsAt: String) {
        self.allDay = allDay
        self.endsAt = endsAt
        self.startsAt = startsAt
    }
}
