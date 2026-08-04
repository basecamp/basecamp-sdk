// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ReplaceScheduleEntryRequest: Codable, Sendable {
    public var allDay: Bool?
    public var description: String?
    public let endsAt: String
    public var highlighted: Bool?
    public var notify: Bool?
    public var participantIds: [Int]?
    public let startsAt: String
    public var summary: String?
    public var url: String?

    public init(
        allDay: Bool? = nil,
        description: String? = nil,
        endsAt: String,
        highlighted: Bool? = nil,
        notify: Bool? = nil,
        participantIds: [Int]? = nil,
        startsAt: String,
        summary: String? = nil,
        url: String? = nil
    ) {
        self.allDay = allDay
        self.description = description
        self.endsAt = endsAt
        self.highlighted = highlighted
        self.notify = notify
        self.participantIds = participantIds
        self.startsAt = startsAt
        self.summary = summary
        self.url = url
    }
}
