// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CampfireLineAttachment: Codable, Sendable {
    public var byteSize: Int?
    public var contentType: String?
    public var downloadUrl: String?
    public var filename: String?
    public var title: String?
    public var url: String?

    public init(
        byteSize: Int? = nil,
        contentType: String? = nil,
        downloadUrl: String? = nil,
        filename: String? = nil,
        title: String? = nil,
        url: String? = nil
    ) {
        self.byteSize = byteSize
        self.contentType = contentType
        self.downloadUrl = downloadUrl
        self.filename = filename
        self.title = title
        self.url = url
    }
}
