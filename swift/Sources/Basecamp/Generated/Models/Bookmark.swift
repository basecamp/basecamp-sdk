// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Bookmark: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public let recording: Recording
    public let updatedAt: String

    public init(
        createdAt: String,
        id: Int,
        recording: Recording,
        updatedAt: String
    ) {
        self.createdAt = createdAt
        self.id = id
        self.recording = recording
        self.updatedAt = updatedAt
    }
}
