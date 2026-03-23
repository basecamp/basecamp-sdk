// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateMyProfileRequest: Codable, Sendable {
    public var bio: String?
    public var emailAddress: String?
    public var firstWeekDay: FirstWeekDay?
    public var location: String?
    public var name: String?
    public var timeFormat: String?
    public var timeZoneName: String?
    public var title: String?

    public init(
        bio: String? = nil,
        emailAddress: String? = nil,
        firstWeekDay: FirstWeekDay? = nil,
        location: String? = nil,
        name: String? = nil,
        timeFormat: String? = nil,
        timeZoneName: String? = nil,
        title: String? = nil
    ) {
        self.bio = bio
        self.emailAddress = emailAddress
        self.firstWeekDay = firstWeekDay
        self.location = location
        self.name = name
        self.timeFormat = timeFormat
        self.timeZoneName = timeZoneName
        self.title = title
    }
}
