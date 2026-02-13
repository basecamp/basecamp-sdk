// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Chatbot: Codable, Sendable {
    public let createdAt: String
    public let id: Int
    public let serviceName: String
    public let updatedAt: String
    public var appUrl: String?
    public var commandUrl: String?
    public var linesUrl: String?
    public var url: String?

    public init(
        createdAt: String,
        id: Int,
        serviceName: String,
        updatedAt: String,
        appUrl: String? = nil,
        commandUrl: String? = nil,
        linesUrl: String? = nil,
        url: String? = nil
    ) {
        self.createdAt = createdAt
        self.id = id
        self.serviceName = serviceName
        self.updatedAt = updatedAt
        self.appUrl = appUrl
        self.commandUrl = commandUrl
        self.linesUrl = linesUrl
        self.url = url
    }
}
