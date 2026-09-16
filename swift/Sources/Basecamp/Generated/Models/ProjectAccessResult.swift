// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ProjectAccessResult: Codable, Sendable {
    public var granted: [Person]?
    public var revoked: [Person]?

    public init(granted: [Person]? = nil, revoked: [Person]? = nil) {
        self.granted = granted
        self.revoked = revoked
    }

    enum CodingKeys: String, CodingKey {
        case granted
        case revoked
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.granted = try container.decodePeopleIfPresent([Person].self, forKey: .granted)
        self.revoked = try container.decodePeopleIfPresent([Person].self, forKey: .revoked)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encodeIfPresent(self.granted, forKey: .granted)
        try container.encodeIfPresent(self.revoked, forKey: .revoked)
    }
}
