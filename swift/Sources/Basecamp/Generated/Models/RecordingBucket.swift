// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct RecordingBucket: Codable, Sendable {
    public let id: Int
    public let name: String
    public let type: String

    public init(id: Int, name: String, type: String) {
        self.id = id
        self.name = name
        self.type = type
    }
}
