// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ProjectAccessResult: Codable, Sendable {
    public var granted: [Person]?
    public var revoked: [Person]?

    public init(granted: [Person]? = nil, revoked: [Person]? = nil) {
        self.granted = granted
        self.revoked = revoked
    }
}
