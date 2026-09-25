// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Todo: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
    public let content: String
    public let createdAt: String
    public let creator: Person
    public let descriptionAttachments: [RichTextAttachment]
    public let id: Int
    public let inheritsStatus: Bool
    public let parent: TodoParent
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var assignees: [Person]?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var completed: Bool?
    public var completionSubscribers: [Person]?
    public var completionUrl: String?
    public var description: String?
    public var dueOn: String?
    public var position: Int32?
    public var startsOn: String?
    public var steps: [CardStep]?
    public var subscriptionUrl: String?
    public var subtasksCompletedCount: Int32?
    public var subtasksCount: Int32?
    public var subtasksUrl: String?

    public init(
        appUrl: String,
        bucket: TodoBucket,
        content: String,
        createdAt: String,
        creator: Person,
        descriptionAttachments: [RichTextAttachment],
        id: Int,
        inheritsStatus: Bool,
        parent: TodoParent,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        assignees: [Person]? = nil,
        bookmarkUrl: String? = nil,
        boostsCount: Int32? = nil,
        boostsUrl: String? = nil,
        commentsCount: Int32? = nil,
        commentsUrl: String? = nil,
        completed: Bool? = nil,
        completionSubscribers: [Person]? = nil,
        completionUrl: String? = nil,
        description: String? = nil,
        dueOn: String? = nil,
        position: Int32? = nil,
        startsOn: String? = nil,
        steps: [CardStep]? = nil,
        subscriptionUrl: String? = nil,
        subtasksCompletedCount: Int32? = nil,
        subtasksCount: Int32? = nil,
        subtasksUrl: String? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.content = content
        self.createdAt = createdAt
        self.creator = creator
        self.descriptionAttachments = descriptionAttachments
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.parent = parent
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.assignees = assignees
        self.bookmarkUrl = bookmarkUrl
        self.boostsCount = boostsCount
        self.boostsUrl = boostsUrl
        self.commentsCount = commentsCount
        self.commentsUrl = commentsUrl
        self.completed = completed
        self.completionSubscribers = completionSubscribers
        self.completionUrl = completionUrl
        self.description = description
        self.dueOn = dueOn
        self.position = position
        self.startsOn = startsOn
        self.steps = steps
        self.subscriptionUrl = subscriptionUrl
        self.subtasksCompletedCount = subtasksCompletedCount
        self.subtasksCount = subtasksCount
        self.subtasksUrl = subtasksUrl
    }

    enum CodingKeys: String, CodingKey {
        case appUrl
        case bucket
        case content
        case createdAt
        case creator
        case descriptionAttachments
        case id
        case inheritsStatus
        case parent
        case status
        case title
        case type
        case updatedAt
        case url
        case visibleToClients
        case assignees
        case bookmarkUrl
        case boostsCount
        case boostsUrl
        case commentsCount
        case commentsUrl
        case completed
        case completionSubscribers
        case completionUrl
        case description
        case dueOn
        case position
        case startsOn
        case steps
        case subscriptionUrl
        case subtasksCompletedCount
        case subtasksCount
        case subtasksUrl
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.bucket = try container.decode(TodoBucket.self, forKey: .bucket)
        self.content = try container.decode(String.self, forKey: .content)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.creator = try container.decode(Person.self, forKey: .creator)
        self.descriptionAttachments = try container.decode([RichTextAttachment].self, forKey: .descriptionAttachments)
        self.id = try container.decode(Int.self, forKey: .id)
        self.inheritsStatus = try container.decode(Bool.self, forKey: .inheritsStatus)
        self.parent = try container.decode(TodoParent.self, forKey: .parent)
        self.status = try container.decode(String.self, forKey: .status)
        self.title = try container.decode(String.self, forKey: .title)
        self.type = try container.decode(String.self, forKey: .type)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
        self.url = try container.decode(String.self, forKey: .url)
        self.visibleToClients = try container.decode(Bool.self, forKey: .visibleToClients)
        self.assignees = try container.decodePeopleIfPresent([Person].self, forKey: .assignees)
        self.bookmarkUrl = try container.decodeIfPresent(String.self, forKey: .bookmarkUrl)
        self.boostsCount = try container.decodeIfPresent(Int32.self, forKey: .boostsCount)
        self.boostsUrl = try container.decodeIfPresent(String.self, forKey: .boostsUrl)
        self.commentsCount = try container.decodeIfPresent(Int32.self, forKey: .commentsCount)
        self.commentsUrl = try container.decodeIfPresent(String.self, forKey: .commentsUrl)
        self.completed = try container.decodeIfPresent(Bool.self, forKey: .completed)
        self.completionSubscribers = try container.decodePeopleIfPresent([Person].self, forKey: .completionSubscribers)
        self.completionUrl = try container.decodeIfPresent(String.self, forKey: .completionUrl)
        self.description = try container.decodeIfPresent(String.self, forKey: .description)
        self.dueOn = try container.decodeIfPresent(String.self, forKey: .dueOn)
        self.position = try container.decodeIfPresent(Int32.self, forKey: .position)
        self.startsOn = try container.decodeIfPresent(String.self, forKey: .startsOn)
        self.steps = try container.decodeIfPresent([CardStep].self, forKey: .steps)
        self.subscriptionUrl = try container.decodeIfPresent(String.self, forKey: .subscriptionUrl)
        self.subtasksCompletedCount = try container.decodeIfPresent(Int32.self, forKey: .subtasksCompletedCount)
        self.subtasksCount = try container.decodeIfPresent(Int32.self, forKey: .subtasksCount)
        self.subtasksUrl = try container.decodeIfPresent(String.self, forKey: .subtasksUrl)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.bucket, forKey: .bucket)
        try container.encode(self.content, forKey: .content)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.creator, forKey: .creator)
        try container.encode(self.descriptionAttachments, forKey: .descriptionAttachments)
        try container.encode(self.id, forKey: .id)
        try container.encode(self.inheritsStatus, forKey: .inheritsStatus)
        try container.encode(self.parent, forKey: .parent)
        try container.encode(self.status, forKey: .status)
        try container.encode(self.title, forKey: .title)
        try container.encode(self.type, forKey: .type)
        try container.encode(self.updatedAt, forKey: .updatedAt)
        try container.encode(self.url, forKey: .url)
        try container.encode(self.visibleToClients, forKey: .visibleToClients)
        try container.encodeIfPresent(self.assignees, forKey: .assignees)
        try container.encodeIfPresent(self.bookmarkUrl, forKey: .bookmarkUrl)
        try container.encodeIfPresent(self.boostsCount, forKey: .boostsCount)
        try container.encodeIfPresent(self.boostsUrl, forKey: .boostsUrl)
        try container.encodeIfPresent(self.commentsCount, forKey: .commentsCount)
        try container.encodeIfPresent(self.commentsUrl, forKey: .commentsUrl)
        try container.encodeIfPresent(self.completed, forKey: .completed)
        try container.encodeIfPresent(self.completionSubscribers, forKey: .completionSubscribers)
        try container.encodeIfPresent(self.completionUrl, forKey: .completionUrl)
        try container.encodeIfPresent(self.description, forKey: .description)
        try container.encodeIfPresent(self.dueOn, forKey: .dueOn)
        try container.encodeIfPresent(self.position, forKey: .position)
        try container.encodeIfPresent(self.startsOn, forKey: .startsOn)
        try container.encodeIfPresent(self.steps, forKey: .steps)
        try container.encodeIfPresent(self.subscriptionUrl, forKey: .subscriptionUrl)
        try container.encodeIfPresent(self.subtasksCompletedCount, forKey: .subtasksCompletedCount)
        try container.encodeIfPresent(self.subtasksCount, forKey: .subtasksCount)
        try container.encodeIfPresent(self.subtasksUrl, forKey: .subtasksUrl)
    }
}
