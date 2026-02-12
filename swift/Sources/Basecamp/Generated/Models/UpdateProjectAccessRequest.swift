// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateProjectAccessRequest: Codable, Sendable {
    public var create: [CreatePersonRequest]?
    public var grant: [Int]?
    public var revoke: [Int]?

    public init(create: [CreatePersonRequest]? = nil, grant: [Int]? = nil, revoke: [Int]? = nil) {
        self.create = create
        self.grant = grant
        self.revoke = revoke
    }
}
