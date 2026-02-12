// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateQuestionRequest: Codable, Sendable {
    public let schedule: QuestionSchedule
    public let title: String

    public init(schedule: QuestionSchedule, title: String) {
        self.schedule = schedule
        self.title = title
    }
}
