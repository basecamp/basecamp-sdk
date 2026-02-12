// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateCardStepRequest: Codable, Sendable {
    public var assignees: [Int]?
    public var dueOn: String?
    public var title: String?

    public init(assignees: [Int]? = nil, dueOn: String? = nil, title: String? = nil) {
        self.assignees = assignees
        self.dueOn = dueOn
        self.title = title
    }
}
