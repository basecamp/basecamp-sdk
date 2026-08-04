// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpcomingScheduleEntry: Codable, Sendable {
    public let allDay: Bool
    public let appUrl: String
    public let bucket: UpcomingScheduleBucket
    public let commentsCount: Int32
    public let creator: UpcomingSchedulePerson
    public let endsAt: String
    public let id: Int
    public let participants: [UpcomingSchedulePerson]
    public let recurring: Bool
    public let startsAt: String
    public let status: String
    public let summary: String
    public let type: String
    public let url: String
    public let visibleToClients: Bool

    public init(
        allDay: Bool,
        appUrl: String,
        bucket: UpcomingScheduleBucket,
        commentsCount: Int32,
        creator: UpcomingSchedulePerson,
        endsAt: String,
        id: Int,
        participants: [UpcomingSchedulePerson],
        recurring: Bool,
        startsAt: String,
        status: String,
        summary: String,
        type: String,
        url: String,
        visibleToClients: Bool
    ) {
        self.allDay = allDay
        self.appUrl = appUrl
        self.bucket = bucket
        self.commentsCount = commentsCount
        self.creator = creator
        self.endsAt = endsAt
        self.id = id
        self.participants = participants
        self.recurring = recurring
        self.startsAt = startsAt
        self.status = status
        self.summary = summary
        self.type = type
        self.url = url
        self.visibleToClients = visibleToClients
    }
}
