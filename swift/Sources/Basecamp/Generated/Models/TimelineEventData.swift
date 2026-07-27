// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct TimelineEventData: Codable, Sendable {
    public let allDay: Bool
    public let endsAt: String?
    public let startsAt: String?

    public init(allDay: Bool, endsAt: String?, startsAt: String?) {
        self.allDay = allDay
        self.endsAt = endsAt
        self.startsAt = startsAt
    }

    enum CodingKeys: String, CodingKey {
        case allDay
        case endsAt
        case startsAt
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.allDay = try container.decode(Bool.self, forKey: .allDay)
        self.endsAt = try container.decode(String?.self, forKey: .endsAt)
        self.startsAt = try container.decode(String?.self, forKey: .startsAt)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.allDay, forKey: .allDay)
        try container.encode(self.endsAt, forKey: .endsAt)
        try container.encode(self.startsAt, forKey: .startsAt)
    }
}
