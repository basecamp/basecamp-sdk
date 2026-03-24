// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateMyPreferencesRequest: Codable, Sendable {
    public let person: PreferencesPayload

    public init(person: PreferencesPayload) {
        self.person = person
    }
}
