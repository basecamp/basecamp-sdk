// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateCardStepRequest: Codable, Sendable {
    public var assigneeIds: [Int]?
    public var dueOn: String?
    public var title: String?

    public init(assigneeIds: [Int]? = nil, dueOn: String? = nil, title: String? = nil) {
        self.assigneeIds = assigneeIds
        self.dueOn = dueOn
        self.title = title
    }
}
