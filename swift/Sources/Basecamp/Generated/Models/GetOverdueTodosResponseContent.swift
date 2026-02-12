// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetOverdueTodosResponseContent: Codable, Sendable {
    public var overAMonthLate: [Todo]?
    public var overAWeekLate: [Todo]?
    public var overThreeMonthsLate: [Todo]?
    public var underAWeekLate: [Todo]?
}
