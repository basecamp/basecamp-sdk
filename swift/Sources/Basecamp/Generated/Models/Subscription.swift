// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Subscription: Codable, Sendable {
    public var count: Int32?
    public var subscribed: Bool?
    public var subscribers: [Person]?
    public var url: String?
}
