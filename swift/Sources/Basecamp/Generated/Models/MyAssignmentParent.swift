// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignmentParent: Codable, Sendable {
    public let id: Int
    public let title: String
    public var appUrl: String?

    public init(id: Int, title: String, appUrl: String? = nil) {
        self.id = id
        self.title = title
        self.appUrl = appUrl
    }
}
