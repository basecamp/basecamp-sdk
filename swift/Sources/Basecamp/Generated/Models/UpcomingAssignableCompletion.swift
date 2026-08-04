// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpcomingAssignableCompletion: Codable, Sendable {
    public let createdAt: String
    public let creator: UpcomingSchedulePerson

    public init(createdAt: String, creator: UpcomingSchedulePerson) {
        self.createdAt = createdAt
        self.creator = creator
    }
}
