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

    public init(
        copy: WebhookCopy? = nil,
        createdAt: String? = nil,
        creator: Person? = nil,
        details: String? = nil,
        id: Int? = nil,
        kind: String? = nil,
        recording: Recording? = nil
    ) {
        self.copy = copy
        self.createdAt = createdAt
        self.creator = creator
        self.details = details
        self.id = id
        self.kind = kind
        self.recording = recording
    }
}
