// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateDoorPayload: Codable, Sendable {
    public var description: String?
    public var service: String?
    public var title: String?
    public var url: String?

    public init(
        description: String? = nil,
        service: String? = nil,
        title: String? = nil,
        url: String? = nil
    ) {
        self.description = description
        self.service = service
        self.title = title
        self.url = url
    }
}
