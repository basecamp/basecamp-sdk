// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Event: Codable, Sendable {
    public var action: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var createdAt: String?
    public var creator: Person?
    public var details: EventDetails?
    public var id: Int?
    public var recordingId: Int?
}
