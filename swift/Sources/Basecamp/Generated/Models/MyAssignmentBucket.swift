// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignmentBucket: Codable, Sendable {
    public let id: Int
    public let name: String
    public var appUrl: String?

    public init(id: Int, name: String, appUrl: String? = nil) {
        self.id = id
        self.name = name
        self.appUrl = appUrl
    }
}
