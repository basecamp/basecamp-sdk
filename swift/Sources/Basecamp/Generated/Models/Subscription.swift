// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Subscription: Codable, Sendable {
    public let count: Int32
    public let subscribed: Bool
    public let url: String
    public var subscribers: [Person]?

    public init(
        count: Int32,
        subscribed: Bool,
        url: String,
        subscribers: [Person]? = nil
    ) {
        self.count = count
        self.subscribed = subscribed
        self.url = url
        self.subscribers = subscribers
    }
}
