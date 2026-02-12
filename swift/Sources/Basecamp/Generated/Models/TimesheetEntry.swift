// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct TimesheetEntry: Codable, Sendable {
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucket: TodoBucket?
    public var createdAt: String?
    public var creator: Person?
    public var date: String?
    public var description: String?
    public var hours: String?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var person: Person?
    public var status: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
