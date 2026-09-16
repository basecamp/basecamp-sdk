// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Event: Codable, Sendable {
    public let action: String
    public let createdAt: String
    public let creator: Person
    public let id: Int
    public let recordingId: Int
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var details: EventDetails?
    public var performedBy: Person?

    public init(
        action: String,
        createdAt: String,
        creator: Person,
        id: Int,
        recordingId: Int,
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        details: EventDetails? = nil,
        performedBy: Person? = nil
    ) {
        self.action = action
        self.createdAt = createdAt
        self.creator = creator
        self.id = id
        self.recordingId = recordingId
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.details = details
        self.performedBy = performedBy
    }
}
