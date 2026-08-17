// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct QuestionSchedule: Codable, Sendable {
    public var days: [Int32]?
    public var endDate: String?
    public var frequency: String?
    public var hour: Int32?
    public var minute: Int32?
    public var monthInterval: Int32?
    public var startDate: String?
    public var weekInstance: Int32?
    public var weekInterval: Int32?

    public init(
        days: [Int32]? = nil,
        endDate: String? = nil,
        frequency: String? = nil,
        hour: Int32? = nil,
        minute: Int32? = nil,
        monthInterval: Int32? = nil,
        startDate: String? = nil,
        weekInstance: Int32? = nil,
        weekInterval: Int32? = nil
    ) {
        self.days = days
        self.endDate = endDate
        self.frequency = frequency
        self.hour = hour
        self.minute = minute
        self.monthInterval = monthInterval
        self.startDate = startDate
        self.weekInstance = weekInstance
        self.weekInterval = weekInterval
    }
}
