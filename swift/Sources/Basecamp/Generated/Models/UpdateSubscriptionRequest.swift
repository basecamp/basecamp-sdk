// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateSubscriptionRequest: Codable, Sendable {
    public var subscriptions: [Int]?
    public var unsubscriptions: [Int]?

    public init(subscriptions: [Int]? = nil, unsubscriptions: [Int]? = nil) {
        self.subscriptions = subscriptions
        self.unsubscriptions = unsubscriptions
    }
}
