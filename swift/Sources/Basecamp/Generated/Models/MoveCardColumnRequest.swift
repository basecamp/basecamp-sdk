// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MoveCardColumnRequest: Codable, Sendable {
    public var position: Int32?
    public let sourceId: Int
    public let targetId: Int

    public init(position: Int32? = nil, sourceId: Int, targetId: Int) {
        self.position = position
        self.sourceId = sourceId
        self.targetId = targetId
    }
}
