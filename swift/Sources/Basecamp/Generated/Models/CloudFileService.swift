// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CloudFileService: Codable, Sendable {
    public let code: String
    public let exampleUrl: String
    public let name: String
    public let validPatterns: [String]
    public var supportingText: String?

    public init(
        code: String,
        exampleUrl: String,
        name: String,
        validPatterns: [String],
        supportingText: String? = nil
    ) {
        self.code = code
        self.exampleUrl = exampleUrl
        self.name = name
        self.validPatterns = validPatterns
        self.supportingText = supportingText
    }
}
