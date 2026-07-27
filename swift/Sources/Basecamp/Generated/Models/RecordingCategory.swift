// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct RecordingCategory: Codable, Sendable {
    public let id: Int
    public let name: String
    public var icon: String?

    public init(id: Int, name: String, icon: String? = nil) {
        self.id = id
        self.name = name
        self.icon = icon
    }
}
