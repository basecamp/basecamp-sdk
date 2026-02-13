// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreatePersonRequest: Codable, Sendable {
    public let emailAddress: String
    public let name: String
    public var companyName: String?
    public var title: String?

    public init(
        emailAddress: String,
        name: String,
        companyName: String? = nil,
        title: String? = nil
    ) {
        self.emailAddress = emailAddress
        self.name = name
        self.companyName = companyName
        self.title = title
    }
}
