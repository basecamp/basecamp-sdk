// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetUpcomingScheduleResponseContent: Codable, Sendable {
    public let assignables: [UpcomingAssignable]
    public let recurringScheduleEntryOccurrences: [UpcomingScheduleEntry]
    public let scheduleEntries: [UpcomingScheduleEntry]

    public init(assignables: [UpcomingAssignable], recurringScheduleEntryOccurrences: [UpcomingScheduleEntry], scheduleEntries: [UpcomingScheduleEntry]) {
        self.assignables = assignables
        self.recurringScheduleEntryOccurrences = recurringScheduleEntryOccurrences
        self.scheduleEntries = scheduleEntries
    }
}
