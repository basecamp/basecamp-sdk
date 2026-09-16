// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Subscription: Codable, Sendable {
    public let count: Int32
    public let subscribed: Bool
    public let url: String
    public var subscribers: [Person]?

    public init(
        count: Int32,
        subscribed: Bool,
        url: String,
        subscribers: [Person]? = nil
    ) {
        self.count = count
        self.subscribed = subscribed
        self.url = url
        self.subscribers = subscribers
    }

    enum CodingKeys: String, CodingKey {
        case count
        case subscribed
        case url
        case subscribers
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.count = try container.decode(Int32.self, forKey: .count)
        self.subscribed = try container.decode(Bool.self, forKey: .subscribed)
        self.url = try container.decode(String.self, forKey: .url)
        self.subscribers = try container.decodePeopleIfPresent([Person].self, forKey: .subscribers)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.count, forKey: .count)
        try container.encode(self.subscribed, forKey: .subscribed)
        try container.encode(self.url, forKey: .url)
        try container.encodeIfPresent(self.subscribers, forKey: .subscribers)
    }
}
