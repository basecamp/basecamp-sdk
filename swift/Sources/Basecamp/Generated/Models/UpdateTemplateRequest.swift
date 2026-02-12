// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateTemplateRequest: Codable, Sendable {
    public var description: String?
    public var name: String?

    public init(description: String? = nil, name: String? = nil) {
        self.description = description
        self.name = name
    }
}
