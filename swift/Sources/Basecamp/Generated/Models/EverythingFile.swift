// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct EverythingFile: Codable, Sendable {
    public var appDownloadUrl: String?
    public var appUrl: String?
    public var attachableSgid: String?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var bucket: RecordingBucket?
    public var byteSize: Int?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var content: String?
    public var contentAttachments: [RichTextAttachment]?
    public var contentType: String?
    public var createdAt: String?
    public var creator: Person?
    public var description: String?
    public var descriptionAttachments: [RichTextAttachment]?
    public var downloadUrl: String?
    public var filename: String?
    public var height: Int32?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var position: Int32?
    public var status: String?
    public var subscriptionUrl: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
    public var width: Int32?
}
