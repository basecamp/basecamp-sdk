// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateProjectClientAccessRequest: Codable, Sendable {
    public var create: [CreateClientRequest]?
    public var grant: [Int]?
    public var revoke: [Int]?

    public init(create: [CreateClientRequest]? = nil, grant: [Int]? = nil, revoke: [Int]? = nil) {
        self.create = create
        self.grant = grant
        self.revoke = revoke
    }
}
