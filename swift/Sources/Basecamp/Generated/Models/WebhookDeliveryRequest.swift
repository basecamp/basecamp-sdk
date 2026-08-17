// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct WebhookDeliveryRequest: Codable, Sendable {
    public var body: WebhookEvent?
    public var headers: WebhookHeadersMap?

    public init(body: WebhookEvent? = nil, headers: WebhookHeadersMap? = nil) {
        self.body = body
        self.headers = headers
    }
}
