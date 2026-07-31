// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateCalendarRequest: Codable, Sendable {
    public let calendar: CalendarAttributes

    public init(calendar: CalendarAttributes) {
        self.calendar = calendar
    }
}
