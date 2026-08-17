// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct PreviewableAttachment: Codable, Sendable {
    public var appUrl: String?
    public var contentType: String?
    public var filename: String?
    public var filesize: Int?
    public var height: Int32?
    public var id: Int?
    public var url: String?
    public var width: Int32?

    public init(
        appUrl: String? = nil,
        contentType: String? = nil,
        filename: String? = nil,
        filesize: Int? = nil,
        height: Int32? = nil,
        id: Int? = nil,
        url: String? = nil,
        width: Int32? = nil
    ) {
        self.appUrl = appUrl
        self.contentType = contentType
        self.filename = filename
        self.filesize = filesize
        self.height = height
        self.id = id
        self.url = url
        self.width = width
    }
}
