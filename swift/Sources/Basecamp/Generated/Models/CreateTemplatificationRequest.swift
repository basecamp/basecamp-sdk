// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateTemplatificationRequest: Codable, Sendable {
    public var copyAssignments: Bool?
    public var copyComments: Bool?
    public var moveCardsToTriage: Bool?
    public var templateName: String?

    public init(
        copyAssignments: Bool? = nil,
        copyComments: Bool? = nil,
        moveCardsToTriage: Bool? = nil,
        templateName: String? = nil
    ) {
        self.copyAssignments = copyAssignments
        self.copyComments = copyComments
        self.moveCardsToTriage = moveCardsToTriage
        self.templateName = templateName
    }
}
