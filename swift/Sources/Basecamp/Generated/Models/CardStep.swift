// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CardStep: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
    public let createdAt: String
    public let creator: Person
    public let id: Int
    public let inheritsStatus: Bool
    public let parent: RecordingParent
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var assignees: [Person]?
    public var bookmarkUrl: String?
    public var completed: Bool?
    public var completedAt: String?
    public var completer: Person?
    public var completionUrl: String?
    public var dueOn: String?
    public var position: Int32?

    public init(
        appUrl: String,
        bucket: TodoBucket,
        createdAt: String,
        creator: Person,
        id: Int,
        inheritsStatus: Bool,
        parent: RecordingParent,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        assignees: [Person]? = nil,
        bookmarkUrl: String? = nil,
        completed: Bool? = nil,
        completedAt: String? = nil,
        completer: Person? = nil,
        completionUrl: String? = nil,
        dueOn: String? = nil,
        position: Int32? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
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
        self.completed = completed
        self.completedAt = completedAt
        self.completer = completer
        self.completionUrl = completionUrl
        self.dueOn = dueOn
        self.position = position
    }

    enum CodingKeys: String, CodingKey {
        case appUrl
        case bucket
        case createdAt
        case creator
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
        case completed
        case completedAt
        case completer
        case completionUrl
        case dueOn
        case position
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.appUrl = try container.decode(String.self, forKey: .appUrl)
        self.bucket = try container.decode(TodoBucket.self, forKey: .bucket)
        self.createdAt = try container.decode(String.self, forKey: .createdAt)
        self.creator = try container.decode(Person.self, forKey: .creator)
        self.id = try container.decode(Int.self, forKey: .id)
        self.inheritsStatus = try container.decode(Bool.self, forKey: .inheritsStatus)
        self.parent = try container.decode(RecordingParent.self, forKey: .parent)
        self.status = try container.decode(String.self, forKey: .status)
        self.title = try container.decode(String.self, forKey: .title)
        self.type = try container.decode(String.self, forKey: .type)
        self.updatedAt = try container.decode(String.self, forKey: .updatedAt)
        self.url = try container.decode(String.self, forKey: .url)
        self.visibleToClients = try container.decode(Bool.self, forKey: .visibleToClients)
        self.assignees = try container.decodePeopleIfPresent([Person].self, forKey: .assignees)
        self.bookmarkUrl = try container.decodeIfPresent(String.self, forKey: .bookmarkUrl)
        self.completed = try container.decodeIfPresent(Bool.self, forKey: .completed)
        self.completedAt = try container.decodeIfPresent(String.self, forKey: .completedAt)
        self.completer = try container.decodeIfPresent(Person.self, forKey: .completer)
        self.completionUrl = try container.decodeIfPresent(String.self, forKey: .completionUrl)
        self.dueOn = try container.decodeIfPresent(String.self, forKey: .dueOn)
        self.position = try container.decodeIfPresent(Int32.self, forKey: .position)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.appUrl, forKey: .appUrl)
        try container.encode(self.bucket, forKey: .bucket)
        try container.encode(self.createdAt, forKey: .createdAt)
        try container.encode(self.creator, forKey: .creator)
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
        try container.encodeIfPresent(self.completed, forKey: .completed)
        try container.encodeIfPresent(self.completedAt, forKey: .completedAt)
        try container.encodeIfPresent(self.completer, forKey: .completer)
        try container.encodeIfPresent(self.completionUrl, forKey: .completionUrl)
        try container.encodeIfPresent(self.dueOn, forKey: .dueOn)
        try container.encodeIfPresent(self.position, forKey: .position)
    }
}
