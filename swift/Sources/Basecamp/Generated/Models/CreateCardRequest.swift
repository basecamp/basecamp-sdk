// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateCardRequest: Codable, Sendable {
    public var content: String?
    public var dueOn: String?
    public var notify: Bool?
    public let title: String

    public init(
        content: String? = nil,
        dueOn: String? = nil,
        notify: Bool? = nil,
        title: String
    ) {
        self.content = content
        self.dueOn = dueOn
        self.notify = notify
        self.title = title
    }
}
