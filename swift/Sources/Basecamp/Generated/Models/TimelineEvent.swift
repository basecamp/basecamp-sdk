// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct TimelineEvent: Codable, Sendable {
    public var action: String?
    public var appUrl: String?
    public var attachments: [TimelineAttachment]?
    public var avatarsSample: [String]?
    public var bucket: TodoBucket?
    public var createdAt: String?
    public var creator: Person?
    public var data: TimelineEventData?
    public var id: Int?
    public var kind: String?
    public var parentRecordingId: Int?
    public var summaryExcerpt: String?
    public var target: String?
    public var title: String?
    public var url: String?
}
