// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateDocumentRequest: Codable, Sendable {
    public var content: String?
    public var status: String?
    public let title: String

    public init(content: String? = nil, status: String? = nil, title: String) {
        self.content = content
        self.status = status
        self.title = title
    }
}
