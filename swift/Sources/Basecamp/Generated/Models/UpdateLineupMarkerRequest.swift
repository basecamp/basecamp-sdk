// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateLineupMarkerRequest: Codable, Sendable {
    public var date: String?
    public var name: String?

    public init(date: String? = nil, name: String? = nil) {
        self.date = date
        self.name = name
    }
}
