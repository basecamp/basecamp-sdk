// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MoveRecordingToVaultRequest: Codable, Sendable {
    public let parentId: Int
    public var position: Int32?

    public init(parentId: Int, position: Int32? = nil) {
        self.parentId = parentId
        self.position = position
    }
}
