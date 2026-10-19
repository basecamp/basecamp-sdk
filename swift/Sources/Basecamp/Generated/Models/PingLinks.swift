// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct PingLinks: Codable, Sendable {
    public let files: String
    public let lines: String

    public init(files: String, lines: String) {
        self.files = files
        self.lines = lines
    }
}
