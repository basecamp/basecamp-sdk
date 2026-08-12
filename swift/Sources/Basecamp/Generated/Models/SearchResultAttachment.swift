// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct SearchResultAttachment: Codable, Sendable {
    public let byteSize: Int
    public let contentType: String
    public let downloadUrl: String
    public let filename: String
    public var height: Int32?
    public var id: Int?
    public var previewUrl: String?
    public var previewable: Bool?
    public var sgid: String?
    public var thumbnailUrl: String?
    public var title: String?
    public var url: String?
    public var width: Int32?

    public init(
        byteSize: Int,
        contentType: String,
        downloadUrl: String,
        filename: String,
        height: Int32? = nil,
        id: Int? = nil,
        previewUrl: String? = nil,
        previewable: Bool? = nil,
        sgid: String? = nil,
        thumbnailUrl: String? = nil,
        title: String? = nil,
        url: String? = nil,
        width: Int32? = nil
    ) {
        self.byteSize = byteSize
        self.contentType = contentType
        self.downloadUrl = downloadUrl
        self.filename = filename
        self.height = height
        self.id = id
        self.previewUrl = previewUrl
        self.previewable = previewable
        self.sgid = sgid
        self.thumbnailUrl = thumbnailUrl
        self.title = title
        self.url = url
        self.width = width
    }
}
