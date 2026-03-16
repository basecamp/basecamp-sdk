// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CardColumnOnHold: Codable, Sendable {
    public let enabled: Bool
    public var cardsCount: Int32?
    public var cardsUrl: String?
    public var id: Int?

    public init(
        enabled: Bool,
        cardsCount: Int32? = nil,
        cardsUrl: String? = nil,
        id: Int? = nil
    ) {
        self.enabled = enabled
        self.cardsCount = cardsCount
        self.cardsUrl = cardsUrl
        self.id = id
    }
}
