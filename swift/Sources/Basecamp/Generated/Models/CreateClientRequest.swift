// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateClientRequest: Codable, Sendable {
    public let emailAddress: String
    public var companyName: String?
    public var name: String?
    public var title: String?

    public init(
        emailAddress: String,
        companyName: String? = nil,
        name: String? = nil,
        title: String? = nil
    ) {
        self.emailAddress = emailAddress
        self.companyName = companyName
        self.name = name
        self.title = title
    }
}
