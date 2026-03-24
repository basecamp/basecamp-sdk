// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignmentParent: Codable, Sendable {
    public let id: Int
    public var appUrl: String?
    public var title: String?

    public init(id: Int, appUrl: String? = nil, title: String? = nil) {
        self.id = id
        self.appUrl = appUrl
        self.title = title
    }
}
