// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct WebhookDelivery: Codable, Sendable {
    public var createdAt: String?
    public var id: Int?
    public var request: WebhookDeliveryRequest?
    public var response: WebhookDeliveryResponse?

    public init(
        createdAt: String? = nil,
        id: Int? = nil,
        request: WebhookDeliveryRequest? = nil,
        response: WebhookDeliveryResponse? = nil
    ) {
        self.createdAt = createdAt
        self.id = id
        self.request = request
        self.response = response
    }
}
