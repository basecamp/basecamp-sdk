// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Webhook: Codable, Sendable {
    public var active: Bool?
    public var appUrl: String?
    public var createdAt: String?
    public var id: Int?
    public var payloadUrl: String?
    public var recentDeliveries: [WebhookDelivery]?
    public var types: [String]?
    public var updatedAt: String?
    public var url: String?
}
