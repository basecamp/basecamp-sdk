// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct WebhookCopy: Codable, Sendable {
    public var appUrl: String?
    public var bucket: WebhookCopyBucket?
    public var id: Int?
    public var url: String?

    public init(
        appUrl: String? = nil,
        bucket: WebhookCopyBucket? = nil,
        id: Int? = nil,
        url: String? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.id = id
        self.url = url
    }
}
