// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct PreferencesPayload: Codable, Sendable {
    public var firstWeekDay: String?
    public var timeFormat: String?
    public var timeZoneName: String?

    public init(firstWeekDay: String? = nil, timeFormat: String? = nil, timeZoneName: String? = nil) {
        self.firstWeekDay = firstWeekDay
        self.timeFormat = timeFormat
        self.timeZoneName = timeZoneName
    }
}
