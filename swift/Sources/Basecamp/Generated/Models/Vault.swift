// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Vault: Codable, Sendable {
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
    public var bookmarkUrl: String?
    public var documentsCount: Int32?
    public var documentsUrl: String?
    public var parent: RecordingParent?
    public var position: Int32?
    public var uploadsCount: Int32?
    public var uploadsUrl: String?
    public var vaultsCount: Int32?
    public var vaultsUrl: String?

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
        bookmarkUrl: String? = nil,
        documentsCount: Int32? = nil,
        documentsUrl: String? = nil,
        parent: RecordingParent? = nil,
        position: Int32? = nil,
        uploadsCount: Int32? = nil,
        uploadsUrl: String? = nil,
        vaultsCount: Int32? = nil,
        vaultsUrl: String? = nil
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
        self.bookmarkUrl = bookmarkUrl
        self.documentsCount = documentsCount
        self.documentsUrl = documentsUrl
        self.parent = parent
        self.position = position
        self.uploadsCount = uploadsCount
        self.uploadsUrl = uploadsUrl
        self.vaultsCount = vaultsCount
        self.vaultsUrl = vaultsUrl
    }
}
