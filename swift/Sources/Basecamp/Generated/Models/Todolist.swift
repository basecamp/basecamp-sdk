// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Todolist: Codable, Sendable {
    public var appTodosUrl: String?
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var bucket: TodoBucket?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var completed: Bool?
    public var completedRatio: String?
    public var createdAt: String?
    public var creator: Person?
    public var description: String?
    public var groupsUrl: String?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var name: String?
    public var parent: TodoParent?
    public var position: Int32?
    public var status: String?
    public var subscriptionUrl: String?
    public var title: String?
    public var todosUrl: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
