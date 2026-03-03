// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateScheduleEntryRequest: Codable, Sendable {
    public var allDay: Bool?
    public var description: String?
    public let endsAt: String
    public var notify: Bool?
    public var participantIds: [Int]?
    public let startsAt: String
    public var subscriptions: [Int]?
    public let summary: String

    public init(
        allDay: Bool? = nil,
        description: String? = nil,
        endsAt: String,
        notify: Bool? = nil,
        participantIds: [Int]? = nil,
        startsAt: String,
        subscriptions: [Int]? = nil,
        summary: String
    ) {
        self.allDay = allDay
        self.description = description
        self.endsAt = endsAt
        self.notify = notify
        self.participantIds = participantIds
        self.startsAt = startsAt
        self.subscriptions = subscriptions
        self.summary = summary
    }
}
