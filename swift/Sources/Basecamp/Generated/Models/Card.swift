// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Card: Codable, Sendable {
    public var appUrl: String?
    public var assignees: [Person]?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var bucket: TodoBucket?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var completed: Bool?
    public var completedAt: String?
    public var completer: Person?
    public var completionSubscribers: [Person]?
    public var completionUrl: String?
    public var content: String?
    public var createdAt: String?
    public var creator: Person?
    public var description: String?
    public var dueOn: String?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var position: Int32?
    public var status: String?
    public var steps: [CardStep]?
    public var subscriptionUrl: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
