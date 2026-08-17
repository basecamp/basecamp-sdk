// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GetOverdueTodosResponseContent: Codable, Sendable {
    public var overAMonthLate: [Todo]?
    public var overAWeekLate: [Todo]?
    public var overThreeMonthsLate: [Todo]?
    public var underAWeekLate: [Todo]?

    public init(
        overAMonthLate: [Todo]? = nil,
        overAWeekLate: [Todo]? = nil,
        overThreeMonthsLate: [Todo]? = nil,
        underAWeekLate: [Todo]? = nil
    ) {
        self.overAMonthLate = overAMonthLate
        self.overAWeekLate = overAWeekLate
        self.overThreeMonthsLate = overThreeMonthsLate
        self.underAWeekLate = underAWeekLate
    }
}
