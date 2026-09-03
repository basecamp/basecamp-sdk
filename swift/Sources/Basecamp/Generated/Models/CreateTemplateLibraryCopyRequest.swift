// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateTemplateLibraryCopyRequest: Codable, Sendable {
    public var addingPeopleConfirmed: Bool?
    public let destinationParentId: Int
    public let templateRecordingId: Int

    public init(addingPeopleConfirmed: Bool? = nil, destinationParentId: Int, templateRecordingId: Int) {
        self.addingPeopleConfirmed = addingPeopleConfirmed
        self.destinationParentId = destinationParentId
        self.templateRecordingId = templateRecordingId
    }
}
