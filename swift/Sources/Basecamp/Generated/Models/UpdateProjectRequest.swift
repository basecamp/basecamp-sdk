// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateProjectRequest: Codable, Sendable {
    public var admissions: String?
    public var description: String?
    public let name: String
    public var scheduleAttributes: ScheduleAttributes?
    public var wallpaperMode: String?

    public init(
        admissions: String? = nil,
        description: String? = nil,
        name: String,
        scheduleAttributes: ScheduleAttributes? = nil,
        wallpaperMode: String? = nil
    ) {
        self.admissions = admissions
        self.description = description
        self.name = name
        self.scheduleAttributes = scheduleAttributes
        self.wallpaperMode = wallpaperMode
    }
}
