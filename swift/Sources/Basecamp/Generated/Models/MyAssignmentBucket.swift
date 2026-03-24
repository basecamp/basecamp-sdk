// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyAssignmentBucket: Codable, Sendable {
    public let id: Int
    public var appUrl: String?
    public var name: String?

    public init(id: Int, appUrl: String? = nil, name: String? = nil) {
        self.id = id
        self.appUrl = appUrl
        self.name = name
    }
}
