// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateGaugeNeedleRequest: Codable, Sendable {
    public let gaugeNeedle: GaugeNeedleUpdatePayload

    public init(gaugeNeedle: GaugeNeedleUpdatePayload) {
        self.gaugeNeedle = gaugeNeedle
    }
}
