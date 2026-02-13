// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Todolist: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
    public let createdAt: String
    public let creator: Person
    public let id: Int
    public let inheritsStatus: Bool
    public let name: String
    public let parent: TodoParent
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var appTodosUrl: String?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var completed: Bool?
    public var completedRatio: String?
    public var description: String?
    public var groupsUrl: String?
    public var position: Int32?
    public var subscriptionUrl: String?
    public var todosUrl: String?

    public init(
        appUrl: String,
        bucket: TodoBucket,
        createdAt: String,
        creator: Person,
        id: Int,
        inheritsStatus: Bool,
        name: String,
        parent: TodoParent,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        appTodosUrl: String? = nil,
        bookmarkUrl: String? = nil,
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        completed: Bool? = nil,
        completedRatio: String? = nil,
        description: String? = nil,
        groupsUrl: String? = nil,
        position: Int32? = nil,
        subscriptionUrl: String? = nil,
        todosUrl: String? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.name = name
        self.parent = parent
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.appTodosUrl = appTodosUrl
        self.bookmarkUrl = bookmarkUrl
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.completed = completed
        self.completedRatio = completedRatio
        self.description = description
        self.groupsUrl = groupsUrl
        self.position = position
        self.subscriptionUrl = subscriptionUrl
        self.todosUrl = todosUrl
    }
}
