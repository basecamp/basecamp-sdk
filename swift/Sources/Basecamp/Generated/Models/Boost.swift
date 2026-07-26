// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Boost: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public var booster: Person?
    public var content: String?
    public var recording: BoostRecording?

    public init(
        createdAt: String,
        id: Int,
        booster: Person? = nil,
        content: String? = nil,
        recording: BoostRecording? = nil
    ) {
        self.createdAt = createdAt
        self.id = id
        self.booster = booster
        self.content = content
        self.recording = recording
    }
}
