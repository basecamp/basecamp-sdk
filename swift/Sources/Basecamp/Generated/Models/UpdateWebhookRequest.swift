// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateWebhookRequest: Codable, Sendable {
    public var active: Bool?
    public var payloadUrl: String?
    public var types: [String]?

    public init(active: Bool? = nil, payloadUrl: String? = nil, types: [String]? = nil) {
        self.active = active
        self.payloadUrl = payloadUrl
        self.types = types
    }
}
