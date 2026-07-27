// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct RecordingParent: Codable, Sendable {
    public let appUrl: String
    public let id: Int
    public let title: String
    public let type: String
    public let url: String
    public var bucket: RecordingBucket?

    public init(
        appUrl: String,
        id: Int,
        title: String,
        type: String,
        url: String,
        bucket: RecordingBucket? = nil
    ) {
        self.appUrl = appUrl
        self.id = id
        self.title = title
        self.type = type
        self.url = url
        self.bucket = bucket
    }
}
