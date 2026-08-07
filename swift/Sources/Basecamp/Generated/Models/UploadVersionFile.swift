// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UploadVersionFile: Codable, Sendable {
    public let appDownloadUrl: String
    public let current: Bool
    public let downloadUrl: String
    public let filename: String
    public var byteSize: Int?
    public var contentType: String?

    public init(
        appDownloadUrl: String,
        current: Bool,
        downloadUrl: String,
        filename: String,
        byteSize: Int? = nil,
        contentType: String? = nil
    ) {
        self.appDownloadUrl = appDownloadUrl
        self.current = current
        self.downloadUrl = downloadUrl
        self.filename = filename
        self.byteSize = byteSize
        self.contentType = contentType
    }
}
