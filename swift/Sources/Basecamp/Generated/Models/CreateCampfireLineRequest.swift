// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateCampfireLineRequest: Codable, Sendable {
    public let content: String
    public var contentType: String?

    public init(content: String, contentType: String? = nil) {
        self.content = content
        self.contentType = contentType
    }
}
