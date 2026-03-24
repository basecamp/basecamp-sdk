// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateGaugeNeedleRequest: Codable, Sendable {
    public let gaugeNeedle: GaugeNeedlePayload
    public var notify: String?
    public var subscriptions: [Int]?

    public init(gaugeNeedle: GaugeNeedlePayload, notify: String? = nil, subscriptions: [Int]? = nil) {
        self.gaugeNeedle = gaugeNeedle
        self.notify = notify
        self.subscriptions = subscriptions
    }
}
