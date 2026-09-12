// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateTemplateLibraryCopyRequest: Codable, Sendable {
    public var addingPeopleConfirmed: Bool?
    public var destinationParentId: Int?
    public var destinationProjectId: Int?
    public let templateRecordingId: Int

    public init(
        addingPeopleConfirmed: Bool? = nil,
        destinationParentId: Int? = nil,
        destinationProjectId: Int? = nil,
        templateRecordingId: Int
    ) {
        self.addingPeopleConfirmed = addingPeopleConfirmed
        self.destinationParentId = destinationParentId
        self.destinationProjectId = destinationProjectId
        self.templateRecordingId = templateRecordingId
    }
}
