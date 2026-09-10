// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ProjectConstructionAttributes: Codable, Sendable {
    public let name: String
    public var description: String?
    public var startDate: String?

    public init(name: String, description: String? = nil, startDate: String? = nil) {
        self.name = name
        self.description = description
        self.startDate = startDate
    }
}
