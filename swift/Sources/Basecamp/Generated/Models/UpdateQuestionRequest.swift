// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateQuestionRequest: Codable, Sendable {
    public var paused: Bool?
    public var schedule: QuestionSchedule?
    public var title: String?

    public init(paused: Bool? = nil, schedule: QuestionSchedule? = nil, title: String? = nil) {
        self.paused = paused
        self.schedule = schedule
        self.title = title
    }
}
