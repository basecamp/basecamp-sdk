// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateChatbotRequest: Codable, Sendable {
    public var commandUrl: String?
    public let serviceName: String

    public init(commandUrl: String? = nil, serviceName: String) {
        self.commandUrl = commandUrl
        self.serviceName = serviceName
    }
}
