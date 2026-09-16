// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct PollInboxResponseContent: Codable, Sendable {
    public let items: [InboxItem]
    public let position: String
    public var next: String?

    public init(items: [InboxItem], position: String, next: String? = nil) {
        self.items = items
        self.position = position
        self.next = next
    }
}
