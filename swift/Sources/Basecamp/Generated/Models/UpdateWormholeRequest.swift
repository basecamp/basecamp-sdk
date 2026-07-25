// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateWormholeRequest: Codable, Sendable {
    public let destinationRecordingId: Int

    public init(destinationRecordingId: Int) {
        self.destinationRecordingId = destinationRecordingId
    }
}
