// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MessageBoard: Codable, Sendable {
    public let appUrl: String
    public let bucket: TodoBucket
    public let createdAt: String
    public let creator: Person
    public let id: Int
    public let inheritsStatus: Bool
    public let status: String
    public let title: String
    public let type: String
    public let updatedAt: String
    public let url: String
    public let visibleToClients: Bool
    public var appMessagesUrl: String?
    public var bookmarkUrl: String?
    public var messagesCount: Int32?
    public var messagesUrl: String?
    public var position: Int32?

    public init(
        appUrl: String,
        bucket: TodoBucket,
        createdAt: String,
        creator: Person,
        id: Int,
        inheritsStatus: Bool,
        status: String,
        title: String,
        type: String,
        updatedAt: String,
        url: String,
        visibleToClients: Bool,
        appMessagesUrl: String? = nil,
        bookmarkUrl: String? = nil,
        messagesCount: Int32? = nil,
        messagesUrl: String? = nil,
        position: Int32? = nil
    ) {
        self.appUrl = appUrl
        self.bucket = bucket
        self.createdAt = createdAt
        self.creator = creator
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.url = url
        self.visibleToClients = visibleToClients
        self.appMessagesUrl = appMessagesUrl
        self.bookmarkUrl = bookmarkUrl
        self.messagesCount = messagesCount
        self.messagesUrl = messagesUrl
        self.position = position
    }
}
