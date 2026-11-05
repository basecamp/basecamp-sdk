// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateDoorRequest: Codable, Sendable {
    public var door: UpdateDoorPayload?
    public var visibleToClients: Bool?

    public init(door: UpdateDoorPayload? = nil, visibleToClients: Bool? = nil) {
        self.door = door
        self.visibleToClients = visibleToClients
    }
}
