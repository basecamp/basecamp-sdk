// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateCloudFileRequest: Codable, Sendable {
    public var description: String?
    public var notify: String?
    public let service: String
    public var subscriptions: [Int]?
    public var title: String?
    public let url: String
    public var visibleToClients: Bool?

    public init(
        description: String? = nil,
        notify: String? = nil,
        service: String,
        subscriptions: [Int]? = nil,
        title: String? = nil,
        url: String,
        visibleToClients: Bool? = nil
    ) {
        self.description = description
        self.notify = notify
        self.service = service
        self.subscriptions = subscriptions
        self.title = title
        self.url = url
        self.visibleToClients = visibleToClients
    }
}
