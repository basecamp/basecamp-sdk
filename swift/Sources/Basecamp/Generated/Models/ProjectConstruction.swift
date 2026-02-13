// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ProjectConstruction: Codable, Sendable {
    public let id: Int
    public let status: String
    public var project: Project?
    public var url: String?

    public init(
        id: Int,
        status: String,
        project: Project? = nil,
        url: String? = nil
    ) {
        self.id = id
        self.status = status
        self.project = project
        self.url = url
    }
}
