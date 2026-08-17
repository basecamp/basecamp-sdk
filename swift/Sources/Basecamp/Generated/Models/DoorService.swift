// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct DoorService: Codable, Sendable {
    public var code: String?
    public var exampleUrl: String?
    public var name: String?
    public var supportingText: String?
    public var validPatterns: [String]?

    public init(
        code: String? = nil,
        exampleUrl: String? = nil,
        name: String? = nil,
        supportingText: String? = nil,
        validPatterns: [String]? = nil
    ) {
        self.code = code
        self.exampleUrl = exampleUrl
        self.name = name
        self.supportingText = supportingText
        self.validPatterns = validPatterns
    }
}
