// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateFolderRequest: Codable, Sendable {
    public var name: String?
    public var projectIds: [Int]?

    public init(name: String? = nil, projectIds: [Int]? = nil) {
        self.name = name
        self.projectIds = projectIds
    }
}
