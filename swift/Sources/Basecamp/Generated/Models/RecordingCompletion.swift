// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct RecordingCompletion: Codable, Sendable {
    public let createdAt: String
    public let creator: Person

    public init(createdAt: String, creator: Person) {
        self.createdAt = createdAt
        self.creator = creator
    }
}
