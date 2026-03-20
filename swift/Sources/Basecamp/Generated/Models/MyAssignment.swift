// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignment: Codable, Sendable {
    public let id: Int
    public var appUrl: String?
    public var assignees: [MyAssignmentPerson]?
    public var bucket: MyAssignmentBucket?
    public var children: [MyAssignment]?
    public var commentsCount: Int32?
    public var completed: Bool?
    public var content: String?
    public var dueOn: String?
    public var hasDescription: Bool?
    public var parent: MyAssignmentParent?
    public var priorityRecordingId: Int?
    public var startsOn: String?
    public var type: String?

    public init(
        id: Int,
        appUrl: String? = nil,
        assignees: [MyAssignmentPerson]? = nil,
        bucket: MyAssignmentBucket? = nil,
        children: [MyAssignment]? = nil,
        commentsCount: Int32? = nil,
        completed: Bool? = nil,
        content: String? = nil,
        dueOn: String? = nil,
        hasDescription: Bool? = nil,
        parent: MyAssignmentParent? = nil,
        priorityRecordingId: Int? = nil,
        startsOn: String? = nil,
        type: String? = nil
    ) {
        self.id = id
        self.appUrl = appUrl
        self.assignees = assignees
        self.bucket = bucket
        self.children = children
        self.commentsCount = commentsCount
        self.completed = completed
        self.content = content
        self.dueOn = dueOn
        self.hasDescription = hasDescription
        self.parent = parent
        self.priorityRecordingId = priorityRecordingId
        self.startsOn = startsOn
        self.type = type
    }
}
