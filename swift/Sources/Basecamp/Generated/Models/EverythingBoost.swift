// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct EverythingBoost: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public var booster: Person?
    public var content: String?
    public var recording: Recording?

    public init(
        createdAt: String,
        id: Int,
        booster: Person? = nil,
        content: String? = nil,
        recording: Recording? = nil
    ) {
        self.createdAt = createdAt
        self.id = id
        self.booster = booster
        self.content = content
        self.recording = recording
    }
}
