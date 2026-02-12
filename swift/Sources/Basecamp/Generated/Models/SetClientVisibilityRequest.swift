// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct SetClientVisibilityRequest: Codable, Sendable {
    public let visibleToClients: Bool

    public init(visibleToClients: Bool) {
        self.visibleToClients = visibleToClients
    }
}
