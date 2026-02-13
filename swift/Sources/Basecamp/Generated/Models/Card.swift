// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Card: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
    public let createdAt: String
    public let creator: Person
    public let id: Int
    public let inheritsStatus: Bool
    public let parent: RecordingParent
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var assignees: [Person]?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var completed: Bool?
    public var completedAt: String?
    public var completer: Person?
    public var completionSubscribers: [Person]?
    public var completionUrl: String?
    public var content: String?
    public var description: String?
    public var dueOn: String?
    public var position: Int32?
    public var steps: [CardStep]?
    public var subscriptionUrl: String?

    public init(
        appUrl: String,
        bucket: TodoBucket,
        createdAt: String,
        creator: Person,
        id: Int,
        inheritsStatus: Bool,
        parent: RecordingParent,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        assignees: [Person]? = nil,
        bookmarkUrl: String? = nil,
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        completed: Bool? = nil,
        completedAt: String? = nil,
        completer: Person? = nil,
        completionSubscribers: [Person]? = nil,
        completionUrl: String? = nil,
        content: String? = nil,
        description: String? = nil,
        dueOn: String? = nil,
        position: Int32? = nil,
        steps: [CardStep]? = nil,
        subscriptionUrl: String? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.parent = parent
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.assignees = assignees
        self.bookmarkUrl = bookmarkUrl
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.completed = completed
        self.completedAt = completedAt
        self.completer = completer
        self.completionSubscribers = completionSubscribers
        self.completionUrl = completionUrl
        self.content = content
        self.description = description
        self.dueOn = dueOn
        self.position = position
        self.steps = steps
        self.subscriptionUrl = subscriptionUrl
    }
}
