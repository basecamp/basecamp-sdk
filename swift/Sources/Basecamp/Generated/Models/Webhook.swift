// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Webhook: Codable, Sendable {
    public let appUrl: String
    public let createdAt: String
    public let id: Int
    public let payloadUrl: String
    public let updatedAt: String
    public let url: String
    public var active: Bool?
    public var recentDeliveries: [WebhookDelivery]?
    public var types: [String]?

    public init(
        appUrl: String,
        createdAt: String,
        id: Int,
        payloadUrl: String,
        updatedAt: String,
        url: String,
        active: Bool? = nil,
        recentDeliveries: [WebhookDelivery]? = nil,
        types: [String]? = nil
    ) {
        self.appUrl = appUrl
        self.createdAt = createdAt
        self.id = id
        self.payloadUrl = payloadUrl
        self.updatedAt = updatedAt
        self.url = url
        self.active = active
        self.recentDeliveries = recentDeliveries
        self.types = types
    }
}
