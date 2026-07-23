// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct SearchType: Codable, Sendable {
    public let key: String?
    public let value: String

    public init(key: String?, value: String) {
        self.key = key
        self.value = value
    }

    enum CodingKeys: String, CodingKey {
        case key
        case value
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.key = try container.decode(String?.self, forKey: .key)
        self.value = try container.decode(String.self, forKey: .value)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.key, forKey: .key)
        try container.encode(self.value, forKey: .value)
    }
}
