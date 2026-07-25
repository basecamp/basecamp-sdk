// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateWormholeRequest: Codable, Sendable {
    public let destinationRecordingId: Int

    public init(destinationRecordingId: Int) {
        self.destinationRecordingId = destinationRecordingId
    }
}
