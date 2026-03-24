// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct EnableOutOfOfficeRequest: Codable, Sendable {
    public let outOfOffice: OutOfOfficePayload

    public init(outOfOffice: OutOfOfficePayload) {
        self.outOfOffice = outOfOffice
    }
}
