// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateDocumentRequest: Codable, Sendable {
    public var content: String?
    public var status: String?
    public var subscriptions: [Int]?
    public let title: String
    public var visibleToClients: Bool?

    public init(
        content: String? = nil,
        status: String? = nil,
        subscriptions: [Int]? = nil,
        title: String,
        visibleToClients: Bool? = nil
    ) {
        self.content = content
        self.status = status
        self.subscriptions = subscriptions
        self.title = title
        self.visibleToClients = visibleToClients
    }
}
