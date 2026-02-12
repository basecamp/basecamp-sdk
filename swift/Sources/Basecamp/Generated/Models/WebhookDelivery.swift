// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct WebhookDelivery: Codable, Sendable {
    public var createdAt: String?
    public var id: Int?
    public var request: WebhookDeliveryRequest?
    public var response: WebhookDeliveryResponse?
}
