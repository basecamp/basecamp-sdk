// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ClientApproval: Codable, Sendable {
    public let appUrl: String
    public let bucket: RecordingBucket
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
    public var approvalStatus: String?
    public var approver: Person?
    public var bookmarkUrl: String?
    public var content: String?
    public var dueOn: String?
    public var repliesCount: Int32?
    public var repliesUrl: String?
    public var responses: [ClientApprovalResponse]?
    public var subject: String?
    public var subscriptionUrl: String?

    public init(
        appUrl: String,
        bucket: RecordingBucket,
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
        approvalStatus: String? = nil,
        approver: Person? = nil,
        bookmarkUrl: String? = nil,
        content: String? = nil,
        dueOn: String? = nil,
        repliesCount: Int32? = nil,
        repliesUrl: String? = nil,
        responses: [ClientApprovalResponse]? = nil,
        subject: String? = nil,
        subscriptionUrl: String? = nil
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
        self.approvalStatus = approvalStatus
        self.approver = approver
        self.bookmarkUrl = bookmarkUrl
        self.content = content
        self.dueOn = dueOn
        self.repliesCount = repliesCount
        self.repliesUrl = repliesUrl
        self.responses = responses
        self.subject = subject
        self.subscriptionUrl = subscriptionUrl
    }
}
