// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateGaugeNeedleRequest: Codable, Sendable {
    public var gaugeNeedle: GaugeNeedleUpdatePayload?

    public init(gaugeNeedle: GaugeNeedleUpdatePayload? = nil) {
        self.gaugeNeedle = gaugeNeedle
    }
}
