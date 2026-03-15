// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CloneToolRequest: Codable, Sendable {
    public let sourceRecordingId: Int
    public var title: String?

    public init(sourceRecordingId: Int, title: String? = nil) {
        self.sourceRecordingId = sourceRecordingId
        self.title = title
    }
}
