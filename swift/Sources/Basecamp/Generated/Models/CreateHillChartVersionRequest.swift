// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateHillChartVersionRequest: Codable, Sendable {
    public var description: String?
    public var dots: [HillChartDotPosition]?
    public var subscriptions: [Int]?

    public init(description: String? = nil, dots: [HillChartDotPosition]? = nil, subscriptions: [Int]? = nil) {
        self.description = description
        self.dots = dots
        self.subscriptions = subscriptions
    }
}
