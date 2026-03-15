// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CloneToolRequest: Codable, Sendable {
    public let sourceRecordingId: Int
    public let title: String

    public init(sourceRecordingId: Int, title: String) {
        self.sourceRecordingId = sourceRecordingId
        self.title = title
    }
}
