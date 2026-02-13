// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct RecordingParent: Codable, Sendable {
    public let appUrl: String
    public let id: Int
    public let title: String
    public let type: String
    public let url: String

    public init(
        appUrl: String,
        id: Int,
        title: String,
        type: String,
        url: String
    ) {
        self.appUrl = appUrl
        self.id = id
        self.title = title
        self.type = type
        self.url = url
    }
}
