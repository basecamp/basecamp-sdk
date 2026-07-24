// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateTodolistRequest: Codable, Sendable {
    public var description: String?
    public let name: String
    public var visibleToClients: Bool?

    public init(description: String? = nil, name: String, visibleToClients: Bool? = nil) {
        self.description = description
        self.name = name
        self.visibleToClients = visibleToClients
    }
}
