// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateTemplateLibraryTodolistRequest: Codable, Sendable {
    public var description: String?
    public let name: String

    public init(description: String? = nil, name: String) {
        self.description = description
        self.name = name
    }
}
