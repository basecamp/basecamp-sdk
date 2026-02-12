// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateTimesheetEntryRequest: Codable, Sendable {
    public var date: String?
    public var description: String?
    public var hours: String?
    public var personId: Int?

    public init(
        date: String? = nil,
        description: String? = nil,
        hours: String? = nil,
        personId: Int? = nil
    ) {
        self.date = date
        self.description = description
        self.hours = hours
        self.personId = personId
    }
}
