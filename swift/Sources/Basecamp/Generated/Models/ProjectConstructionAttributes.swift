// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ProjectConstructionAttributes: Codable, Sendable {
    public let name: String
    public var description: String?

    public init(name: String, description: String? = nil) {
        self.name = name
        self.description = description
    }
}
