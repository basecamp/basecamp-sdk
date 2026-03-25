// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct OutOfOfficePerson: Codable, Sendable {
    public let id: Int
    public var name: String?

    public init(id: Int, name: String? = nil) {
        self.id = id
        self.name = name
    }
}
