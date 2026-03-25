// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MoveCardRequest: Codable, Sendable {
    public let columnId: Int
    public var position: Int32?

    public init(columnId: Int, position: Int32? = nil) {
        self.columnId = columnId
        self.position = position
    }
}
