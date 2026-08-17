// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct QuestionReminder: Codable, Sendable {
    public var groupOn: String?
    public var question: Question?
    public var remindAt: String?
    public var reminderId: Int?

    public init(
        groupOn: String? = nil,
        question: Question? = nil,
        remindAt: String? = nil,
        reminderId: Int? = nil
    ) {
        self.groupOn = groupOn
        self.question = question
        self.remindAt = remindAt
        self.reminderId = reminderId
    }
}
