// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignment: Codable, Sendable {
    public let appUrl: String
    public let assignees: [MyAssignmentPerson]
    public let bucket: MyAssignmentBucket
    public let children: [MyAssignment]
    public let commentsCount: Int32
    public let completed: Bool
    public let content: String
    public let hasDescription: Bool
    public let id: Int
    public let parent: MyAssignmentParent
    public let type: String
    public var dueOn: String?
    public var priorityRecordingId: Int?
    public var startsOn: String?

    public init(
        appUrl: String,
        assignees: [MyAssignmentPerson],
        bucket: MyAssignmentBucket,
        children: [MyAssignment],
        commentsCount: Int32,
        completed: Bool,
        content: String,
        hasDescription: Bool,
        id: Int,
        parent: MyAssignmentParent,
        type: String,
        dueOn: String? = nil,
        priorityRecordingId: Int? = nil,
        startsOn: String? = nil
    ) {
        self.appUrl = appUrl
        self.assignees = assignees
        self.bucket = bucket
        self.children = children
        self.commentsCount = commentsCount
        self.completed = completed
        self.content = content
        self.hasDescription = hasDescription
        self.id = id
        self.parent = parent
        self.type = type
        self.dueOn = dueOn
        self.priorityRecordingId = priorityRecordingId
        self.startsOn = startsOn
    }
}
