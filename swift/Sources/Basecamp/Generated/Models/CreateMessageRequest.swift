// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateMessageRequest: Codable, Sendable {
    public var categoryId: Int?
    public var content: String?
    public var status: String?
    public let subject: String

    public init(
        categoryId: Int? = nil,
        content: String? = nil,
        status: String? = nil,
        subject: String
    ) {
        self.categoryId = categoryId
        self.content = content
        self.status = status
        self.subject = subject
    }
}
