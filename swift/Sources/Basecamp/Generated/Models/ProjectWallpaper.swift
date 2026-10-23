// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ProjectWallpaper: Codable, Sendable {
    public let mode: String
    public let url: String

    public init(mode: String, url: String) {
        self.mode = mode
        self.url = url
    }
}
