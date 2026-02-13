// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Todoset: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
    public let createdAt: String
    public let creator: Person
    public let id: Int
    public let inheritsStatus: Bool
    public let name: String
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var appTodolistsUrl: String?
    public var bookmarkUrl: String?
    public var completed: Bool?
    public var completedCount: Int32?
    public var completedRatio: String?
    public var onScheduleCount: Int32?
    public var overScheduleCount: Int32?
    public var position: Int32?
    public var todolistsCount: Int32?
    public var todolistsUrl: String?

    public init(
        appUrl: String,
        bucket: TodoBucket,
        createdAt: String,
        creator: Person,
        id: Int,
        inheritsStatus: Bool,
        name: String,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        appTodolistsUrl: String? = nil,
        bookmarkUrl: String? = nil,
        completed: Bool? = nil,
        completedCount: Int32? = nil,
        completedRatio: String? = nil,
        onScheduleCount: Int32? = nil,
        overScheduleCount: Int32? = nil,
        position: Int32? = nil,
        todolistsCount: Int32? = nil,
        todolistsUrl: String? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.name = name
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.appTodolistsUrl = appTodolistsUrl
        self.bookmarkUrl = bookmarkUrl
        self.completed = completed
        self.completedCount = completedCount
        self.completedRatio = completedRatio
        self.onScheduleCount = onScheduleCount
        self.overScheduleCount = overScheduleCount
        self.position = position
        self.todolistsCount = todolistsCount
        self.todolistsUrl = todolistsUrl
    }
}
