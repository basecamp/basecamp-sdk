// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct QuestionScheduleInput: Codable, Sendable {
    public var days: [Int32]?
    public var frequency: String?
    public var startDate: String?
    public var timeOfDay: String?
    public var weekInstance: Int32?

    public init(
        days: [Int32]? = nil,
        frequency: String? = nil,
        startDate: String? = nil,
        timeOfDay: String? = nil,
        weekInstance: Int32? = nil
    ) {
        self.days = days
        self.frequency = frequency
        self.startDate = startDate
        self.timeOfDay = timeOfDay
        self.weekInstance = weekInstance
    }
}
