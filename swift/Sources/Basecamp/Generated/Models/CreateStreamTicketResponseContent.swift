// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateStreamTicketResponseContent: Codable, Sendable {
    public let expiresIn: Int32
    public let ticket: String
    public let url: String

    public init(expiresIn: Int32, ticket: String, url: String) {
        self.expiresIn = expiresIn
        self.ticket = ticket
        self.url = url
    }
}
