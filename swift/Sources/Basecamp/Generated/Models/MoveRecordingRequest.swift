// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MoveRecordingRequest: Codable, Sendable {
    public var destinationProjectId: Int?
    public var filingDestinationId: Int?
    public var parentId: Int?

    public init(destinationProjectId: Int? = nil, filingDestinationId: Int? = nil, parentId: Int? = nil) {
        self.destinationProjectId = destinationProjectId
        self.filingDestinationId = filingDestinationId
        self.parentId = parentId
    }
}
