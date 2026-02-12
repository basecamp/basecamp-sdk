// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateWebhookRequest: Codable, Sendable {
    public var active: Bool?
    public let payloadUrl: String
    public let types: [String]

    public init(active: Bool? = nil, payloadUrl: String, types: [String]) {
        self.active = active
        self.payloadUrl = payloadUrl
        self.types = types
    }
}
