// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct WebhookEvent: Codable, Sendable {
    public var copy: WebhookCopy?
    public var createdAt: String?
    public var creator: Person?
    public var details: String?
    public var id: Int?
    public var kind: String?
    public var recording: Recording?
}
