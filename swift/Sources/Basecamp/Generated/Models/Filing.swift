// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Filing: Codable, Sendable {
    public let id: Int
    public let sourceRecordingId: Int
    public let status: String
    public let type: String
    public let url: String
    public var destinationRecording: FilingDestinationRecording?

    public init(
        id: Int,
        sourceRecordingId: Int,
        status: String,
        type: String,
        url: String,
        destinationRecording: FilingDestinationRecording? = nil
    ) {
        self.id = id
        self.sourceRecordingId = sourceRecordingId
        self.status = status
        self.type = type
        self.url = url
        self.destinationRecording = destinationRecording
    }
}
