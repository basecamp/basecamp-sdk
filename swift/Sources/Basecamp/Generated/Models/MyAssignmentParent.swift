// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignmentParent: Codable, Sendable {
    public let appUrl: String
    public let id: Int
    public let title: String

    public init(appUrl: String, id: Int, title: String) {
        self.appUrl = appUrl
        self.id = id
        self.title = title
    }
}
