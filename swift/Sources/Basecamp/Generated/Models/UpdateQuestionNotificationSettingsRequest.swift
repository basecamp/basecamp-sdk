// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateQuestionNotificationSettingsRequest: Codable, Sendable {
    public var digestIncludeUnanswered: Bool?
    public var notifyOnAnswer: Bool?

    public init(digestIncludeUnanswered: Bool? = nil, notifyOnAnswer: Bool? = nil) {
        self.digestIncludeUnanswered = digestIncludeUnanswered
        self.notifyOnAnswer = notifyOnAnswer
    }
}
