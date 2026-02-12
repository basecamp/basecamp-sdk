// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateTodoRequest: Codable, Sendable {
    public var assigneeIds: [Int]?
    public var completionSubscriberIds: [Int]?
    public let content: String
    public var description: String?
    public var dueOn: String?
    public var notify: Bool?
    public var startsOn: String?

    public init(
        assigneeIds: [Int]? = nil,
        completionSubscriberIds: [Int]? = nil,
        content: String,
        description: String? = nil,
        dueOn: String? = nil,
        notify: Bool? = nil,
        startsOn: String? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.completionSubscriberIds = completionSubscriberIds
        self.content = content
        self.description = description
        self.dueOn = dueOn
        self.notify = notify
        self.startsOn = startsOn
    }
}
