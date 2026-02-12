// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Schedule: Codable, Sendable {
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucket: TodoBucket?
    public var createdAt: String?
    public var creator: Person?
    public var entriesCount: Int32?
    public var entriesUrl: String?
    public var id: Int?
    public var includeDueAssignments: Bool?
    public var inheritsStatus: Bool?
    public var position: Int32?
    public var status: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
