// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct OutOfOffice: Codable, Sendable {
    public var enabled: Bool?
    public var endDate: String?
    public var ongoing: Bool?
    public var person: OutOfOfficePerson?
    public var startDate: String?
}
