// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct GaugeNeedlePayload: Codable, Sendable {
    public let position: Int32
    public var color: String?
    public var description: String?

    public init(position: Int32, color: String? = nil, description: String? = nil) {
        self.position = position
        self.color = color
        self.description = description
    }
}
