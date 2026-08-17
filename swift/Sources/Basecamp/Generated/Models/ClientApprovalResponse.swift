// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ClientApprovalResponse: Codable, Sendable {
    public var appUrl: String?
    public var approved: Bool?
    public var bookmarkUrl: String?
    public var bucket: RecordingBucket?
    public var content: String?
    public var createdAt: String?
    public var creator: Person?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var status: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var visibleToClients: Bool?

    public init(
        appUrl: String? = nil,
        approved: Bool? = nil,
        bookmarkUrl: String? = nil,
        bucket: RecordingBucket? = nil,
        content: String? = nil,
        createdAt: String? = nil,
        creator: Person? = nil,
        id: Int? = nil,
        inheritsStatus: Bool? = nil,
        parent: RecordingParent? = nil,
        status: String? = nil,
        title: String? = nil,
        type: String? = nil,
        updatedAt: String? = nil,
        visibleToClients: Bool? = nil
    ) {
        self.appUrl = appUrl
        self.approved = approved
        self.bookmarkUrl = bookmarkUrl
        self.bucket = bucket
        self.content = content
        self.createdAt = createdAt
        self.creator = creator
        self.id = id
        self.inheritsStatus = inheritsStatus
        self.parent = parent
        self.status = status
        self.title = title
        self.type = type
        self.updatedAt = updatedAt
        self.visibleToClients = visibleToClients
    }
}
