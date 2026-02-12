// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateCardColumnRequest: Codable, Sendable {
    public var description: String?
    public var title: String?

    public init(description: String? = nil, title: String? = nil) {
        self.description = description
        self.title = title
    }
}
