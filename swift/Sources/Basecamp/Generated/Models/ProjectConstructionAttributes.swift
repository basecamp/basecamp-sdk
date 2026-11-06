// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ProjectConstructionAttributes: Codable, Sendable {
    public let name: String
    public var admissions: String?
    public var description: String?
    public var startDate: String?
    public var timesheetEnabled: Bool?

    public init(
        name: String,
        admissions: String? = nil,
        description: String? = nil,
        startDate: String? = nil,
        timesheetEnabled: Bool? = nil
    ) {
        self.name = name
        self.admissions = admissions
        self.description = description
        self.startDate = startDate
        self.timesheetEnabled = timesheetEnabled
    }
}
