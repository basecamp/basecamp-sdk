// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateDocumentRequest: Codable, Sendable {
    public var content: String?
    public var title: String?

    public init(content: String? = nil, title: String? = nil) {
        self.content = content
        self.title = title
    }
}
