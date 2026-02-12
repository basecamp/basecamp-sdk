// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateUploadRequest: Codable, Sendable {
    public let attachableSgid: String
    public var baseName: String?
    public var description: String?

    public init(attachableSgid: String, baseName: String? = nil, description: String? = nil) {
        self.attachableSgid = attachableSgid
        self.baseName = baseName
        self.description = description
    }
}
