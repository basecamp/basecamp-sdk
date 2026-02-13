// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CardStep: Codable, Sendable {
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
    public var completed: Bool?
    public var completedAt: String?
    public var completer: Person?
    public var completionUrl: String?
    public var dueOn: String?
    public var position: Int32?

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
        completed: Bool? = nil,
        completedAt: String? = nil,
        completer: Person? = nil,
        completionUrl: String? = nil,
        dueOn: String? = nil,
        position: Int32? = nil
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
        self.completed = completed
        self.completedAt = completedAt
        self.completer = completer
        self.completionUrl = completionUrl
        self.dueOn = dueOn
        self.position = position
    }
}
