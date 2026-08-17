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

    public init(
        appDownloadUrl: String? = nil,
        appUrl: String? = nil,
        attachableSgid: String? = nil,
        bookmarkUrl: String? = nil,
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        bucket: RecordingBucket? = nil,
        byteSize: Int? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        content: String? = nil,
        contentAttachments: [RichTextAttachment]? = nil,
        contentType: String? = nil,
        createdAt: String? = nil,
        creator: Person? = nil,
        description: String? = nil,
        descriptionAttachments: [RichTextAttachment]? = nil,
        downloadUrl: String? = nil,
        filename: String? = nil,
        height: Int32? = nil,
        id: Int? = nil,
        inheritsStatus: Bool? = nil,
        parent: RecordingParent? = nil,
        position: Int32? = nil,
        status: String? = nil,
        subscriptionUrl: String? = nil,
        title: String? = nil,
        type: String? = nil,
        updatedAt: String? = nil,
        url: String? = nil,
        visibleToClients: Bool? = nil,
        width: Int32? = nil
    ) {
        self.appDownloadUrl = appDownloadUrl
        self.appUrl = appUrl
        self.attachableSgid = attachableSgid
        self.bookmarkUrl = bookmarkUrl
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.bucket = bucket
        self.byteSize = byteSize
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.content = content
        self.contentAttachments = contentAttachments
        self.contentType = contentType
        self.createdAt = createdAt
        self.creator = creator
        self.description = description
        self.descriptionAttachments = descriptionAttachments
        self.downloadUrl = downloadUrl
        self.filename = filename
        self.height = height
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.parent = parent
        self.position = position
        self.status = status
        self.subscriptionUrl = subscriptionUrl
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.width = width
    }
}
