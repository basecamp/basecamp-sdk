// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ToggleGaugeRequest: Codable, Sendable {
    public let gauge: GaugeTogglePayload

    public init(gauge: GaugeTogglePayload) {
        self.gauge = gauge
    }
}
