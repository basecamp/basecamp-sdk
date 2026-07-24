// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateQuestionRequest: Codable, Sendable {
    public let schedule: QuestionSchedule
    public let title: String
    public var visibleToClients: Bool?

    public init(schedule: QuestionSchedule, title: String, visibleToClients: Bool? = nil) {
        self.schedule = schedule
        self.title = title
        self.visibleToClients = visibleToClients
    }
}
