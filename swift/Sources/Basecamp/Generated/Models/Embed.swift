// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Embed: Codable, Sendable {
    public let sgid: String
    public let status: String
    public let url: String
    public var content: String?
    public var contentType: String?
    public var imageUrl: String?
    public var type: String?

    public init(
        sgid: String,
        status: String,
        url: String,
        content: String? = nil,
        contentType: String? = nil,
        imageUrl: String? = nil,
        type: String? = nil
    ) {
        self.sgid = sgid
        self.status = status
        self.url = url
        self.content = content
        self.contentType = contentType
        self.imageUrl = imageUrl
        self.type = type
    }
}
