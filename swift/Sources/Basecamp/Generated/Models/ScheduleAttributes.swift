// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ScheduleAttributes: Codable, Sendable {
    public var endDate: String?
    public var startDate: String?

    public init(endDate: String? = nil, startDate: String? = nil) {
        self.endDate = endDate
        self.startDate = startDate
    }
}
