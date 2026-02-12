// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateMessageRequest: Codable, Sendable {
    public var categoryId: Int?
    public var content: String?
    public var status: String?
    public var subject: String?

    public init(
        categoryId: Int? = nil,
        content: String? = nil,
        status: String? = nil,
        subject: String? = nil
    ) {
        self.categoryId = categoryId
        self.content = content
        self.status = status
        self.subject = subject
    }
}
