// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct EventDetails: Codable, Sendable {
    public var addedPersonIds: [Int]?
    public var notifiedRecipientIds: [Int]?
    public var removedPersonIds: [Int]?

    public init(addedPersonIds: [Int]? = nil, notifiedRecipientIds: [Int]? = nil, removedPersonIds: [Int]? = nil) {
        self.addedPersonIds = addedPersonIds
        self.notifiedRecipientIds = notifiedRecipientIds
        self.removedPersonIds = removedPersonIds
    }
}
