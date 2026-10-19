// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreatePingRequest: Codable, Sendable {
    public var newGroup: Bool?
    public let userIds: [Int]

    public init(newGroup: Bool? = nil, userIds: [Int]) {
        self.newGroup = newGroup
        self.userIds = userIds
    }
}
