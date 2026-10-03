// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateQuestionNotificationSettingsRequest: Codable, Sendable {
    public var responding: Bool?
    public var subscribed: Bool?

    public init(responding: Bool? = nil, subscribed: Bool? = nil) {
        self.responding = responding
        self.subscribed = subscribed
    }
}
