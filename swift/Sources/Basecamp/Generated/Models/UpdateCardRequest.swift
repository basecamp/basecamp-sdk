// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateCardRequest: Codable, Sendable {
    public var assigneeIds: [Int]?
    public var content: String?
    public var dueOn: String?
    public var title: String?

    public init(
        assigneeIds: [Int]? = nil,
        content: String? = nil,
        dueOn: String? = nil,
        title: String? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.content = content
        self.dueOn = dueOn
        self.title = title
    }
}
