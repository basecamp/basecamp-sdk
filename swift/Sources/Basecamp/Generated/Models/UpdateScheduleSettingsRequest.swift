// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateScheduleSettingsRequest: Codable, Sendable {
    public let includeDueAssignments: Bool

    public init(includeDueAssignments: Bool) {
        self.includeDueAssignments = includeDueAssignments
    }
}
