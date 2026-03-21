// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignmentBucket: Codable, Sendable {
    public let appUrl: String
    public let id: Int
    public let name: String

    public init(appUrl: String, id: Int, name: String) {
        self.appUrl = appUrl
        self.id = id
        self.name = name
    }
}
