// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct WebhookDeliveryResponse: Codable, Sendable {
    public var code: Int32?
    public var headers: WebhookHeadersMap?
    public var message: String?

    public init(code: Int32? = nil, headers: WebhookHeadersMap? = nil, message: String? = nil) {
        self.code = code
        self.headers = headers
        self.message = message
    }
}
