// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct QuestionAnswerUpdatePayload: Codable, Sendable {
    public let content: String
    public var groupOn: String?

    public init(content: String, groupOn: String? = nil) {
        self.content = content
        self.groupOn = groupOn
    }
}
