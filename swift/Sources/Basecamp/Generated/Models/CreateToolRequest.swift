// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateToolRequest: Codable, Sendable {
    public var title: String?
    public let toolType: String

    public init(title: String? = nil, toolType: String) {
        self.title = title
        self.toolType = toolType
    }
}
