// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct RepositionCardStepRequest: Codable, Sendable {
    public let position: Int32
    public let sourceId: Int

    public init(position: Int32, sourceId: Int) {
        self.position = position
        self.sourceId = sourceId
    }
}
