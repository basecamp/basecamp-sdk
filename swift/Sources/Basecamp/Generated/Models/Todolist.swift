// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Todolist: Codable, Sendable {
    public let appUrl: String
    public let bubbleUpUrl: String
    public let bucket: TodoBucket
    public let color: String?
    public let commentsAppUrl: String
    public let createdAt: String
    public let creator: Person
    public let description: String
    public let descriptionAttachments: [RichTextAttachment]
    public let id: Int
    public let inheritsStatus: Bool
    public let name: String
    public let parent: TodoParent
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var appTodosUrl: String?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var completed: Bool?
    public var completedRatio: String?
    public var groupPositionUrl: String?
    public var groupsUrl: String?
    public var position: Int32?
    public var subscriptionUrl: String?
    public var todosUrl: String?

    public init(
        appUrl: String,
        bubbleUpUrl: String,
        bucket: TodoBucket,
        color: String?,
        commentsAppUrl: String,
        createdAt: String,
        creator: Person,
        description: String,
        descriptionAttachments: [RichTextAttachment],
        id: Int,
        inheritsStatus: Bool,
        name: String,
        parent: TodoParent,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        appTodosUrl: String? = nil,
        bookmarkUrl: String? = nil,
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        completed: Bool? = nil,
        completedRatio: String? = nil,
        groupPositionUrl: String? = nil,
        groupsUrl: String? = nil,
        position: Int32? = nil,
        subscriptionUrl: String? = nil,
        todosUrl: String? = nil
    ) {
        self.appUrl = appUrl
        self.bubbleUpUrl = bubbleUpUrl
        self.bucket = bucket
        self.color = color
        self.commentsAppUrl = commentsAppUrl
        self.createdAt = createdAt
        self.creator = creator
        self.description = description
        self.descriptionAttachments = descriptionAttachments
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.name = name
        self.parent = parent
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.appTodosUrl = appTodosUrl
        self.bookmarkUrl = bookmarkUrl
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.completed = completed
        self.completedRatio = completedRatio
        self.groupPositionUrl = groupPositionUrl
        self.groupsUrl = groupsUrl
        self.position = position
        self.subscriptionUrl = subscriptionUrl
        self.todosUrl = todosUrl
    }

    enum CodingKeys: String, CodingKey {
        case appUrl
        case bubbleUpUrl
        case bucket
        case color
        case commentsAppUrl
        case createdAt
        case creator
        case description
        case descriptionAttachments
        case id
        case inheritsStatus
        case name
        case parent
        case status
        case title
        case type
        case updatedAt
        case url
        case visibleToClients
        case appTodosUrl
        case bookmarkUrl
        case boostsCount
        case boostsUrl
        case commentsCount
        case commentsUrl
        case completed
        case completedRatio
        case groupPositionUrl
        case groupsUrl
        case position
        case subscriptionUrl
        case todosUrl
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.bubbleUpUrl = try container.decode(String.self, forKey: .bubbleUpUrl)
        self.bucket = try container.decode(TodoBucket.self, forKey: .bucket)
        self.color = try container.decode(String?.self, forKey: .color)
        self.commentsAppUrl = try container.decode(String.self, forKey: .commentsAppUrl)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.creator = try container.decode(Person.self, forKey: .creator)
        self.description = try container.decode(String.self, forKey: .description)
        self.descriptionAttachments = try container.decode([RichTextAttachment].self, forKey: .descriptionAttachments)
        self.id = try container.decode(Int.self, forKey: .id)
        self.inheritsStatus = try container.decode(Bool.self, forKey: .inheritsStatus)
        self.name = try container.decode(String.self, forKey: .name)
        self.parent = try container.decode(TodoParent.self, forKey: .parent)
        self.status = try container.decode(String.self, forKey: .status)
        self.title = try container.decode(String.self, forKey: .title)
        self.type = try container.decode(String.self, forKey: .type)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
        self.url = try container.decode(String.self, forKey: .url)
        self.visibleToClients = try container.decode(Bool.self, forKey: .visibleToClients)
        self.appTodosUrl = try container.decodeIfPresent(String.self, forKey: .appTodosUrl)
        self.bookmarkUrl = try container.decodeIfPresent(String.self, forKey: .bookmarkUrl)
        self.boostsCount = try container.decodeIfPresent(Int32.self, forKey: .boostsCount)
        self.boostsUrl = try container.decodeIfPresent(String.self, forKey: .boostsUrl)
        self.commentsCount = try container.decodeIfPresent(Int32.self, forKey: .commentsCount)
        self.commentsUrl = try container.decodeIfPresent(String.self, forKey: .commentsUrl)
        self.completed = try container.decodeIfPresent(Bool.self, forKey: .completed)
        self.completedRatio = try container.decodeIfPresent(String.self, forKey: .completedRatio)
        self.groupPositionUrl = try container.decodeIfPresent(String.self, forKey: .groupPositionUrl)
        self.groupsUrl = try container.decodeIfPresent(String.self, forKey: .groupsUrl)
        self.position = try container.decodeIfPresent(Int32.self, forKey: .position)
        self.subscriptionUrl = try container.decodeIfPresent(String.self, forKey: .subscriptionUrl)
        self.todosUrl = try container.decodeIfPresent(String.self, forKey: .todosUrl)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.bubbleUpUrl, forKey: .bubbleUpUrl)
        try container.encode(self.bucket, forKey: .bucket)
        try container.encode(self.color, forKey: .color)
        try container.encode(self.commentsAppUrl, forKey: .commentsAppUrl)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.creator, forKey: .creator)
        try container.encode(self.description, forKey: .description)
        try container.encode(self.descriptionAttachments, forKey: .descriptionAttachments)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.inheritsStatus, forKey: .inheritsStatus)
        try container.encode(self.name, forKey: .name)
        try container.encode(self.parent, forKey: .parent)
        try container.encode(self.status, forKey: .status)
        try container.encode(self.title, forKey: .title)
        try container.encode(self.type, forKey: .type)
        try container.encode(self.updatedAt, forKey: .updatedAt)
        try container.encode(self.url, forKey: .url)
        try container.encode(self.visibleToClients, forKey: .visibleToClients)
        try container.encodeIfPresent(self.appTodosUrl, forKey: .appTodosUrl)
        try container.encodeIfPresent(self.bookmarkUrl, forKey: .bookmarkUrl)
        try container.encodeIfPresent(self.boostsCount, forKey: .boostsCount)
        try container.encodeIfPresent(self.boostsUrl, forKey: .boostsUrl)
        try container.encodeIfPresent(self.commentsCount, forKey: .commentsCount)
        try container.encodeIfPresent(self.commentsUrl, forKey: .commentsUrl)
        try container.encodeIfPresent(self.completed, forKey: .completed)
        try container.encodeIfPresent(self.completedRatio, forKey: .completedRatio)
        try container.encodeIfPresent(self.groupPositionUrl, forKey: .groupPositionUrl)
        try container.encodeIfPresent(self.groupsUrl, forKey: .groupsUrl)
        try container.encodeIfPresent(self.position, forKey: .position)
        try container.encodeIfPresent(self.subscriptionUrl, forKey: .subscriptionUrl)
        try container.encodeIfPresent(self.todosUrl, forKey: .todosUrl)
    }
}
