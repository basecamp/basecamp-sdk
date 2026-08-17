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

    public init(
        action: String? = nil,
        appUrl: String? = nil,
        attachments: [TimelineAttachment]? = nil,
        avatarsSample: [String]? = nil,
        bucket: TodoBucket? = nil,
        createdAt: String? = nil,
        creator: Person? = nil,
        data: TimelineEventData? = nil,
        id: Int? = nil,
        kind: String? = nil,
        parentRecordingId: Int? = nil,
        summaryExcerpt: String? = nil,
        target: String? = nil,
        title: String? = nil,
        url: String? = nil
    ) {
        self.action = action
        self.appUrl = appUrl
        self.attachments = attachments
        self.avatarsSample = avatarsSample
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.data = data
        self.id = id
        self.kind = kind
        self.parentRecordingId = parentRecordingId
        self.summaryExcerpt = summaryExcerpt
        self.target = target
        self.title = title
        self.url = url
    }
}
