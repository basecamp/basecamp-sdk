// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MarkAsReadRequest: Codable, Sendable {
    public let readables: [String]

    public init(readables: [String]) {
        self.readables = readables
    }
}
