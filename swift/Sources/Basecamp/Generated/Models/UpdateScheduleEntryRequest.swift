// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateScheduleEntryRequest: Codable, Sendable {
    public var allDay: Bool?
    public var description: String?
    public var endsAt: String?
    public var notify: Bool?
    public var participantIds: [Int]?
    public var startsAt: String?
    public var summary: String?

    public init(
        allDay: Bool? = nil,
        description: String? = nil,
        endsAt: String? = nil,
        notify: Bool? = nil,
        participantIds: [Int]? = nil,
        startsAt: String? = nil,
        summary: String? = nil
    ) {
        self.allDay = allDay
        self.description = description
        self.endsAt = endsAt
        self.notify = notify
        self.participantIds = participantIds
        self.startsAt = startsAt
        self.summary = summary
    }
}
