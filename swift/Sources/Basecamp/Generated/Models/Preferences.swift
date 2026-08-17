// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Preferences: Codable, Sendable {
    public var appUrl: String?
    public var firstWeekDay: String?
    public var timeFormat: String?
    public var timeZoneName: String?
    public var url: String?

    public init(
        appUrl: String? = nil,
        firstWeekDay: String? = nil,
        timeFormat: String? = nil,
        timeZoneName: String? = nil,
        url: String? = nil
    ) {
        self.appUrl = appUrl
        self.firstWeekDay = firstWeekDay
        self.timeFormat = timeFormat
        self.timeZoneName = timeZoneName
        self.url = url
    }
}
