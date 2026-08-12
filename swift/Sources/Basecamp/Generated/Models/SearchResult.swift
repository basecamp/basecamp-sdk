// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct SearchResult: Codable, Sendable {
    public let content: String?
    public let description: String?
    public var appDownloadUrl: String?
    public var appUrl: String?
    public var attachments: [SearchResultAttachment]?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var bubbleUpUrl: String?
    public var bucket: RecordingBucket?
    public var byteSize: Int?
    public var cardsCount: Int32?
    public var cardsUrl: String?
    public var color: String?
    public var commentCount: Int32?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var contentAttachments: [RichTextAttachment]?
    public var contentType: String?
    public var createdAt: String?
    public var creator: Person?
    public var descriptionAttachments: [RichTextAttachment]?
    public var downloadUrl: String?
    public var filename: String?
    public var height: Int32?
    public var id: Int?
    public var imageUrl: String?
    public var inheritsStatus: Bool?
    public var language: String?
    public var onHold: CardColumnOnHold?
    public var parent: RecordingParent?
    public var plainTextContent: String?
    public var plainTextDescription: String?
    public var position: Int32?
    public var previewUrl: String?
    public var previewable: Bool?
    public var soundUrl: String?
    public var status: String?
    public var subject: String?
    public var subscribers: [Person]?
    public var subscriptionUrl: String?
    public var thumbnailUrl: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
    public var width: Int32?

    public init(
        content: String?,
        description: String?,
        appDownloadUrl: String? = nil,
        appUrl: String? = nil,
        attachments: [SearchResultAttachment]? = nil,
        bookmarkUrl: String? = nil,
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        bubbleUpUrl: String? = nil,
        bucket: RecordingBucket? = nil,
        byteSize: Int? = nil,
        cardsCount: Int32? = nil,
        cardsUrl: String? = nil,
        color: String? = nil,
        commentCount: Int32? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        contentAttachments: [RichTextAttachment]? = nil,
        contentType: String? = nil,
        createdAt: String? = nil,
        creator: Person? = nil,
        descriptionAttachments: [RichTextAttachment]? = nil,
        downloadUrl: String? = nil,
        filename: String? = nil,
        height: Int32? = nil,
        id: Int? = nil,
        imageUrl: String? = nil,
        inheritsStatus: Bool? = nil,
        language: String? = nil,
        onHold: CardColumnOnHold? = nil,
        parent: RecordingParent? = nil,
        plainTextContent: String? = nil,
        plainTextDescription: String? = nil,
        position: Int32? = nil,
        previewUrl: String? = nil,
        previewable: Bool? = nil,
        soundUrl: String? = nil,
        status: String? = nil,
        subject: String? = nil,
        subscribers: [Person]? = nil,
        subscriptionUrl: String? = nil,
        thumbnailUrl: String? = nil,
        title: String? = nil,
        type: String? = nil,
        updatedAt: String? = nil,
        url: String? = nil,
        visibleToClients: Bool? = nil,
        width: Int32? = nil
    ) {
        self.content = content
        self.description = description
        self.appDownloadUrl = appDownloadUrl
        self.appUrl = appUrl
        self.attachments = attachments
        self.bookmarkUrl = bookmarkUrl
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.bubbleUpUrl = bubbleUpUrl
        self.bucket = bucket
        self.byteSize = byteSize
        self.cardsCount = cardsCount
        self.cardsUrl = cardsUrl
        self.color = color
        self.commentCount = commentCount
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.contentAttachments = contentAttachments
        self.contentType = contentType
        self.createdAt = createdAt
        self.creator = creator
        self.descriptionAttachments = descriptionAttachments
        self.downloadUrl = downloadUrl
        self.filename = filename
        self.height = height
        self.id = id
        self.imageUrl = imageUrl
        self.inheritsStatus = inheritsStatus
        self.language = language
        self.onHold = onHold
        self.parent = parent
        self.plainTextContent = plainTextContent
        self.plainTextDescription = plainTextDescription
        self.position = position
        self.previewUrl = previewUrl
        self.previewable = previewable
        self.soundUrl = soundUrl
        self.status = status
        self.subject = subject
        self.subscribers = subscribers
        self.subscriptionUrl = subscriptionUrl
        self.thumbnailUrl = thumbnailUrl
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.width = width
    }

    enum CodingKeys: String, CodingKey {
        case content
        case description
        case appDownloadUrl
        case appUrl
        case attachments
        case bookmarkUrl
        case boostsCount
        case boostsUrl
        case bubbleUpUrl
        case bucket
        case byteSize
        case cardsCount
        case cardsUrl
        case color
        case commentCount
        case commentsCount
        case commentsUrl
        case contentAttachments
        case contentType
        case createdAt
        case creator
        case descriptionAttachments
        case downloadUrl
        case filename
        case height
        case id
        case imageUrl
        case inheritsStatus
        case language
        case onHold
        case parent
        case plainTextContent
        case plainTextDescription
        case position
        case previewUrl
        case previewable
        case soundUrl
        case status
        case subject
        case subscribers
        case subscriptionUrl
        case thumbnailUrl
        case title
        case type
        case updatedAt
        case url
        case visibleToClients
        case width
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.content = try container.decode(String?.self, forKey: .content)
        self.description = try container.decode(String?.self, forKey: .description)
        self.appDownloadUrl = try container.decodeIfPresent(String.self, forKey: .appDownloadUrl)
        self.appUrl = try container.decodeIfPresent(String.self, forKey: .appUrl)
        self.attachments = try container.decodeIfPresent([SearchResultAttachment].self, forKey: .attachments)
        self.bookmarkUrl = try container.decodeIfPresent(String.self, forKey: .bookmarkUrl)
        self.boostsCount = try container.decodeIfPresent(Int32.self, forKey: .boostsCount)
        self.boostsUrl = try container.decodeIfPresent(String.self, forKey: .boostsUrl)
        self.bubbleUpUrl = try container.decodeIfPresent(String.self, forKey: .bubbleUpUrl)
        self.bucket = try container.decodeIfPresent(RecordingBucket.self, forKey: .bucket)
        self.byteSize = try container.decodeIfPresent(Int.self, forKey: .byteSize)
        self.cardsCount = try container.decodeIfPresent(Int32.self, forKey: .cardsCount)
        self.cardsUrl = try container.decodeIfPresent(String.self, forKey: .cardsUrl)
        self.color = try container.decodeIfPresent(String.self, forKey: .color)
        self.commentCount = try container.decodeIfPresent(Int32.self, forKey: .commentCount)
        self.commentsCount = try container.decodeIfPresent(Int32.self, forKey: .commentsCount)
        self.commentsUrl = try container.decodeIfPresent(String.self, forKey: .commentsUrl)
        self.contentAttachments = try container.decodeIfPresent([RichTextAttachment].self, forKey: .contentAttachments)
        self.contentType = try container.decodeIfPresent(String.self, forKey: .contentType)
        self.createdAt = try container.decodeIfPresent(String.self, forKey: .createdAt)
        self.creator = try container.decodeIfPresent(Person.self, forKey: .creator)
        self.descriptionAttachments = try container.decodeIfPresent([RichTextAttachment].self, forKey: .descriptionAttachments)
        self.downloadUrl = try container.decodeIfPresent(String.self, forKey: .downloadUrl)
        self.filename = try container.decodeIfPresent(String.self, forKey: .filename)
        self.height = try container.decodeIfPresent(Int32.self, forKey: .height)
        self.id = try container.decodeIfPresent(Int.self, forKey: .id)
        self.imageUrl = try container.decodeIfPresent(String.self, forKey: .imageUrl)
        self.inheritsStatus = try container.decodeIfPresent(Bool.self, forKey: .inheritsStatus)
        self.language = try container.decodeIfPresent(String.self, forKey: .language)
        self.onHold = try container.decodeIfPresent(CardColumnOnHold.self, forKey: .onHold)
        self.parent = try container.decodeIfPresent(RecordingParent.self, forKey: .parent)
        self.plainTextContent = try container.decodeIfPresent(String.self, forKey: .plainTextContent)
        self.plainTextDescription = try container.decodeIfPresent(String.self, forKey: .plainTextDescription)
        self.position = try container.decodeIfPresent(Int32.self, forKey: .position)
        self.previewUrl = try container.decodeIfPresent(String.self, forKey: .previewUrl)
        self.previewable = try container.decodeIfPresent(Bool.self, forKey: .previewable)
        self.soundUrl = try container.decodeIfPresent(String.self, forKey: .soundUrl)
        self.status = try container.decodeIfPresent(String.self, forKey: .status)
        self.subject = try container.decodeIfPresent(String.self, forKey: .subject)
        self.subscribers = try container.decodeIfPresent([Person].self, forKey: .subscribers)
        self.subscriptionUrl = try container.decodeIfPresent(String.self, forKey: .subscriptionUrl)
        self.thumbnailUrl = try container.decodeIfPresent(String.self, forKey: .thumbnailUrl)
        self.title = try container.decodeIfPresent(String.self, forKey: .title)
        self.type = try container.decodeIfPresent(String.self, forKey: .type)
        self.updatedAt = try container.decodeIfPresent(String.self, forKey: .updatedAt)
        self.url = try container.decodeIfPresent(String.self, forKey: .url)
        self.visibleToClients = try container.decodeIfPresent(Bool.self, forKey: .visibleToClients)
        self.width = try container.decodeIfPresent(Int32.self, forKey: .width)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.content, forKey: .content)
        try container.encode(self.description, forKey: .description)
        try container.encodeIfPresent(self.appDownloadUrl, forKey: .appDownloadUrl)
        try container.encodeIfPresent(self.appUrl, forKey: .appUrl)
        try container.encodeIfPresent(self.attachments, forKey: .attachments)
        try container.encodeIfPresent(self.bookmarkUrl, forKey: .bookmarkUrl)
        try container.encodeIfPresent(self.boostsCount, forKey: .boostsCount)
        try container.encodeIfPresent(self.boostsUrl, forKey: .boostsUrl)
        try container.encodeIfPresent(self.bubbleUpUrl, forKey: .bubbleUpUrl)
        try container.encodeIfPresent(self.bucket, forKey: .bucket)
        try container.encodeIfPresent(self.byteSize, forKey: .byteSize)
        try container.encodeIfPresent(self.cardsCount, forKey: .cardsCount)
        try container.encodeIfPresent(self.cardsUrl, forKey: .cardsUrl)
        try container.encodeIfPresent(self.color, forKey: .color)
        try container.encodeIfPresent(self.commentCount, forKey: .commentCount)
        try container.encodeIfPresent(self.commentsCount, forKey: .commentsCount)
        try container.encodeIfPresent(self.commentsUrl, forKey: .commentsUrl)
        try container.encodeIfPresent(self.contentAttachments, forKey: .contentAttachments)
        try container.encodeIfPresent(self.contentType, forKey: .contentType)
        try container.encodeIfPresent(self.createdAt, forKey: .createdAt)
        try container.encodeIfPresent(self.creator, forKey: .creator)
        try container.encodeIfPresent(self.descriptionAttachments, forKey: .descriptionAttachments)
        try container.encodeIfPresent(self.downloadUrl, forKey: .downloadUrl)
        try container.encodeIfPresent(self.filename, forKey: .filename)
        try container.encodeIfPresent(self.height, forKey: .height)
        try container.encodeIfPresent(self.id, forKey: .id)
        try container.encodeIfPresent(self.imageUrl, forKey: .imageUrl)
        try container.encodeIfPresent(self.inheritsStatus, forKey: .inheritsStatus)
        try container.encodeIfPresent(self.language, forKey: .language)
        try container.encodeIfPresent(self.onHold, forKey: .onHold)
        try container.encodeIfPresent(self.parent, forKey: .parent)
        try container.encodeIfPresent(self.plainTextContent, forKey: .plainTextContent)
        try container.encodeIfPresent(self.plainTextDescription, forKey: .plainTextDescription)
        try container.encodeIfPresent(self.position, forKey: .position)
        try container.encodeIfPresent(self.previewUrl, forKey: .previewUrl)
        try container.encodeIfPresent(self.previewable, forKey: .previewable)
        try container.encodeIfPresent(self.soundUrl, forKey: .soundUrl)
        try container.encodeIfPresent(self.status, forKey: .status)
        try container.encodeIfPresent(self.subject, forKey: .subject)
        try container.encodeIfPresent(self.subscribers, forKey: .subscribers)
        try container.encodeIfPresent(self.subscriptionUrl, forKey: .subscriptionUrl)
        try container.encodeIfPresent(self.thumbnailUrl, forKey: .thumbnailUrl)
        try container.encodeIfPresent(self.title, forKey: .title)
        try container.encodeIfPresent(self.type, forKey: .type)
        try container.encodeIfPresent(self.updatedAt, forKey: .updatedAt)
        try container.encodeIfPresent(self.url, forKey: .url)
        try container.encodeIfPresent(self.visibleToClients, forKey: .visibleToClients)
        try container.encodeIfPresent(self.width, forKey: .width)
    }
}
