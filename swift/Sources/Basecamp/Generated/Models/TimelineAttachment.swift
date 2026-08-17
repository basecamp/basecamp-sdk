// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct TimelineAttachment: Codable, Sendable {
    public var appDownloadUrl: String?
    public var appUrl: String?
    public var attachableSgid: String?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var bucket: TodoBucket?
    public var byteSize: Int?
    public var caption: String?
    public var commentsCount: Int32?
    public var commentsUrl: String?
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
    public var key: String?
    public var parent: RecordingParent?
    public var position: Int32?
    public var previewUrl: String?
    public var previewable: Bool?
    public var sgid: String?
    public var status: String?
    public var statusUrl: String?
    public var subscriptionUrl: String?
    public var thumbnailUrl: String?
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
        bucket: TodoBucket? = nil,
        byteSize: Int? = nil,
        caption: String? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
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
        key: String? = nil,
        parent: RecordingParent? = nil,
        position: Int32? = nil,
        previewUrl: String? = nil,
        previewable: Bool? = nil,
        sgid: String? = nil,
        status: String? = nil,
        statusUrl: String? = nil,
        subscriptionUrl: String? = nil,
        thumbnailUrl: String? = nil,
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
        self.caption = caption
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
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
        self.key = key
        self.parent = parent
        self.position = position
        self.previewUrl = previewUrl
        self.previewable = previewable
        self.sgid = sgid
        self.status = status
        self.statusUrl = statusUrl
        self.subscriptionUrl = subscriptionUrl
        self.thumbnailUrl = thumbnailUrl
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.width = width
    }
}
