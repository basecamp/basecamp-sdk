// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct FilingDestinationRecording: Codable, Sendable {
    public let appUrl: String
    public let id: Int
    public let type: String
    public let url: String

    public init(
        appUrl: String,
        id: Int,
        type: String,
        url: String
    ) {
        self.appUrl = appUrl
        self.id = id
        self.type = type
        self.url = url
    }
}
