// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateDoorRequest: Codable, Sendable {
    public let door: CreateDoorPayload
    public var visibleToClients: Bool?

    public init(door: CreateDoorPayload, visibleToClients: Bool? = nil) {
        self.door = door
        self.visibleToClients = visibleToClients
    }
}
