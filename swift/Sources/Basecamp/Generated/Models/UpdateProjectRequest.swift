// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateProjectRequest: Codable, Sendable {
    public var admissions: String?
    public var color: String?
    public var description: String?
    public let name: String
    public var purpose: String?
    public var scheduleAttributes: ScheduleAttributes?
    public var timesheetEnabled: Bool?
    public var wallpaperMode: String?

    public init(
        admissions: String? = nil,
        color: String? = nil,
        description: String? = nil,
        name: String,
        purpose: String? = nil,
        scheduleAttributes: ScheduleAttributes? = nil,
        timesheetEnabled: Bool? = nil,
        wallpaperMode: String? = nil
    ) {
        self.admissions = admissions
        self.color = color
        self.description = description
        self.name = name
        self.purpose = purpose
        self.scheduleAttributes = scheduleAttributes
        self.timesheetEnabled = timesheetEnabled
        self.wallpaperMode = wallpaperMode
    }
}
