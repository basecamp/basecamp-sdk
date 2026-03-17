// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CardColumnOnHold: Codable, Sendable {
    public let cardsCount: Int32
    public let cardsUrl: String
    public let createdAt: String
    public let id: Int
    public let inheritsStatus: Bool
    public let status: String
    public let title: String
    public let updatedAt: String

    public init(
        cardsCount: Int32,
        cardsUrl: String,
        createdAt: String,
        id: Int,
        inheritsStatus: Bool,
        status: String,
        title: String,
        updatedAt: String
    ) {
        self.cardsCount = cardsCount
        self.cardsUrl = cardsUrl
        self.createdAt = createdAt
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.status = status
        self.title = title
        self.updatedAt = updatedAt
    }
}
