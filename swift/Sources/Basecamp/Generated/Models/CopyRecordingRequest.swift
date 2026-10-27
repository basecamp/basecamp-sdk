// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CopyRecordingRequest: Codable, Sendable {
    public var destinationProjectId: Int?
    public var filingDestinationId: Int?
    public var includeComments: Bool?
    public var parentId: Int?

    public init(
        destinationProjectId: Int? = nil,
        filingDestinationId: Int? = nil,
        includeComments: Bool? = nil,
        parentId: Int? = nil
    ) {
        self.destinationProjectId = destinationProjectId
        self.filingDestinationId = filingDestinationId
        self.includeComments = includeComments
        self.parentId = parentId
    }
}
