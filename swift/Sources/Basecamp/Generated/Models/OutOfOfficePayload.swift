// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct OutOfOfficePayload: Codable, Sendable {
    public let endDate: String
    public let startDate: String

    public init(endDate: String, startDate: String) {
        self.endDate = endDate
        self.startDate = startDate
    }
}
