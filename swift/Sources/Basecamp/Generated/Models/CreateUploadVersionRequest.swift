// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateUploadVersionRequest: Codable, Sendable {
    public let attachableSgid: String
    public var baseName: String?
    public var description: String?
    public var notify: String?
    public var subscriptions: [Int]?

    public init(
        attachableSgid: String,
        baseName: String? = nil,
        description: String? = nil,
        notify: String? = nil,
        subscriptions: [Int]? = nil
    ) {
        self.attachableSgid = attachableSgid
        self.baseName = baseName
        self.description = description
        self.notify = notify
        self.subscriptions = subscriptions
    }
}
