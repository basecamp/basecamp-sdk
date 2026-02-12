// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateMessageTypeRequest: Codable, Sendable {
    public var icon: String?
    public var name: String?

    public init(icon: String? = nil, name: String? = nil) {
        self.icon = icon
        self.name = name
    }
}
