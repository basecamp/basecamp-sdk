// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateLineupMarkerRequest: Codable, Sendable {
    public let date: String
    public let name: String

    public init(date: String, name: String) {
        self.date = date
        self.name = name
    }
}
