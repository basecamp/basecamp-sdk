// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateQuestionRequest: Codable, Sendable {
    public var schedule: QuestionScheduleInput?
    public var title: String?

    public init(schedule: QuestionScheduleInput? = nil, title: String? = nil) {
        self.schedule = schedule
        self.title = title
    }
}
