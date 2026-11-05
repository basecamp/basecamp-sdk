// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateDoorPayload: Codable, Sendable {
    public let service: String
    public let url: String
    public var description: String?
    public var title: String?

    public init(
        service: String,
        url: String,
        description: String? = nil,
        title: String? = nil
    ) {
        self.service = service
        self.url = url
        self.description = description
        self.title = title
    }
}
