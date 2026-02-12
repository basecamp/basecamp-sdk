// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateCardColumnRequest: Codable, Sendable {
    public var description: String?
    public let title: String

    public init(description: String? = nil, title: String) {
        self.description = description
        self.title = title
    }
}
