// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateScheduleEntryRequest: Codable, Sendable {
    public var allDay: Bool?
    public var description: String?
    public let endsAt: String
    public var notify: Bool?
    public var participantIds: [Int]?
    public let startsAt: String
    public let summary: String

    public init(
        allDay: Bool? = nil,
        description: String? = nil,
        endsAt: String,
        notify: Bool? = nil,
        participantIds: [Int]? = nil,
        startsAt: String,
        summary: String
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
