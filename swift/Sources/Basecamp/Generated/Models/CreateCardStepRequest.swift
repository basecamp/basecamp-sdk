// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateCardStepRequest: Codable, Sendable {
    public var assignees: [Int]?
    public var dueOn: String?
    public let title: String

    public init(assignees: [Int]? = nil, dueOn: String? = nil, title: String) {
        self.assignees = assignees
        self.dueOn = dueOn
        self.title = title
    }
}
