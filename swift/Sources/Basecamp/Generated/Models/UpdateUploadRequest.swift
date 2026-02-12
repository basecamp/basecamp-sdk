// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateUploadRequest: Codable, Sendable {
    public var baseName: String?
    public var description: String?

    public init(baseName: String? = nil, description: String? = nil) {
        self.baseName = baseName
        self.description = description
    }
}
