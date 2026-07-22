// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct RichTextAttachment: Codable, Sendable {
    public let byteSize: Int
    public let contentType: String
    public let downloadUrl: String
    public let filename: String
    public let id: Int
    public let previewUrl: String
    public let previewable: Bool
    public let sgid: String
    public let thumbnailUrl: String
    public var height: Int32?
    public var width: Int32?

    public init(
        byteSize: Int,
        contentType: String,
        downloadUrl: String,
        filename: String,
        id: Int,
        previewUrl: String,
        previewable: Bool,
        sgid: String,
        thumbnailUrl: String,
        height: Int32? = nil,
        width: Int32? = nil
    ) {
        self.byteSize = byteSize
        self.contentType = contentType
        self.downloadUrl = downloadUrl
        self.filename = filename
        self.id = id
        self.previewUrl = previewUrl
        self.previewable = previewable
        self.sgid = sgid
        self.thumbnailUrl = thumbnailUrl
        self.height = height
        self.width = width
    }
}
