// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateMessageTypeRequest: Codable, Sendable {
    public let icon: String
    public let name: String

    public init(icon: String, name: String) {
        self.icon = icon
        self.name = name
    }
}
