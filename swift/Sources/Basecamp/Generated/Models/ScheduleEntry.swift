// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ScheduleEntry: Codable, Sendable {
    public var allDay: Bool?
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var bucket: TodoBucket?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var createdAt: String?
    public var creator: Person?
    public var description: String?
    public var endsAt: String?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var participants: [Person]?
    public var startsAt: String?
    public var status: String?
    public var subscriptionUrl: String?
    public var summary: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
