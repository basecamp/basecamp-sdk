// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct HillChart: Codable, Sendable {
    public let enabled: Bool
    public let stale: Bool
    public var appUpdateUrl: String?
    public var appVersionsUrl: String?
    public var dots: [HillChartDot]?
    public var updatedAt: String?

    public init(
        enabled: Bool,
        stale: Bool,
        appUpdateUrl: String? = nil,
        appVersionsUrl: String? = nil,
        dots: [HillChartDot]? = nil,
        updatedAt: String? = nil
    ) {
        self.enabled = enabled
        self.stale = stale
        self.appUpdateUrl = appUpdateUrl
        self.appVersionsUrl = appVersionsUrl
        self.dots = dots
        self.updatedAt = updatedAt
    }
}
