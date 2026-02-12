// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateTimesheetEntryRequest: Codable, Sendable {
    public let date: String
    public var description: String?
    public let hours: String
    public var personId: Int?

    public init(
        date: String,
        description: String? = nil,
        hours: String,
        personId: Int? = nil
    ) {
        self.date = date
        self.description = description
        self.hours = hours
        self.personId = personId
    }
}
