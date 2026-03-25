// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Account: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public let name: String
    public let updatedAt: String
    public var active: Bool?
    public var frozen: Bool?
    public var limits: AccountLimits?
    public var logo: AccountLogo?
    public var ownerName: String?
    public var paused: Bool?
    public var settings: AccountSettings?
    public var subscription: AccountSubscription?
    public var trial: Bool?
    public var trialEndsOn: String?

    public init(
        createdAt: String,
        id: Int,
        name: String,
        updatedAt: String,
        active: Bool? = nil,
        frozen: Bool? = nil,
        limits: AccountLimits? = nil,
        logo: AccountLogo? = nil,
        ownerName: String? = nil,
        paused: Bool? = nil,
        settings: AccountSettings? = nil,
        subscription: AccountSubscription? = nil,
        trial: Bool? = nil,
        trialEndsOn: String? = nil
    ) {
        self.createdAt = createdAt
        self.id = id
        self.name = name
        self.updatedAt = updatedAt
        self.active = active
        self.frozen = frozen
        self.limits = limits
        self.logo = logo
        self.ownerName = ownerName
        self.paused = paused
        self.settings = settings
        self.subscription = subscription
        self.trial = trial
        self.trialEndsOn = trialEndsOn
    }
}
