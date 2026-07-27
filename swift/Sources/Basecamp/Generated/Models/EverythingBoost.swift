// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct EverythingBoost: Codable, Sendable {
    public let booster: Person
    public let content: String
    public let createdAt: String
    public let id: Int
    public let recording: Recording

    public init(
        booster: Person,
        content: String,
        createdAt: String,
        id: Int,
        recording: Recording
    ) {
        self.booster = booster
        self.content = content
        self.createdAt = createdAt
        self.id = id
        self.recording = recording
    }
}
