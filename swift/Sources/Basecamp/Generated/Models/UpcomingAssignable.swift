// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpcomingAssignable: Codable, Sendable {
    public let appUrl: String
    public let assignees: [UpcomingSchedulePerson]
    public let bucket: UpcomingScheduleBucket
    public let commentsCount: Int32
    public let completed: Bool
    public let completionUrl: String
    public let content: String
    public let id: Int
    public let parent: UpcomingAssignableParent
    public let repeating: Bool
    public let status: String
    public let type: String
    public let url: String
    public let visibleToClients: Bool
    public var completion: UpcomingAssignableCompletion?
    public var dueOn: String?
    public var startsOn: String?

    public init(
        appUrl: String,
        assignees: [UpcomingSchedulePerson],
        bucket: UpcomingScheduleBucket,
        commentsCount: Int32,
        completed: Bool,
        completionUrl: String,
        content: String,
        id: Int,
        parent: UpcomingAssignableParent,
        repeating: Bool,
        status: String,
        type: String,
        url: String,
        visibleToClients: Bool,
        completion: UpcomingAssignableCompletion? = nil,
        dueOn: String? = nil,
        startsOn: String? = nil
    ) {
        self.appUrl = appUrl
        self.assignees = assignees
        self.bucket = bucket
        self.commentsCount = commentsCount
        self.completed = completed
        self.completionUrl = completionUrl
        self.content = content
        self.id = id
        self.parent = parent
        self.repeating = repeating
        self.status = status
        self.type = type
        self.url = url
        self.visibleToClients = visibleToClients
        self.completion = completion
        self.dueOn = dueOn
        self.startsOn = startsOn
    }
}
