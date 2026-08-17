// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct OutOfOffice: Codable, Sendable {
    public var backOnDate: String?
    public var enabled: Bool?
    public var endDate: String?
    public var ongoing: Bool?
    public var person: OutOfOfficePerson?
    public var startDate: String?

    public init(
        backOnDate: String? = nil,
        enabled: Bool? = nil,
        endDate: String? = nil,
        ongoing: Bool? = nil,
        person: OutOfOfficePerson? = nil,
        startDate: String? = nil
    ) {
        self.backOnDate = backOnDate
        self.enabled = enabled
        self.endDate = endDate
        self.ongoing = ongoing
        self.person = person
        self.startDate = startDate
    }
}
