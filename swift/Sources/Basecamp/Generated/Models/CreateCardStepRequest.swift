// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateCardStepRequest: Codable, Sendable {
    public var assigneeIds: [Int]?
    public var dueOn: String?
    public let title: String

    public init(assigneeIds: [Int]? = nil, dueOn: String? = nil, title: String) {
        self.assigneeIds = assigneeIds
        self.dueOn = dueOn
        self.title = title
    }
}
