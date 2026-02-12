// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Vault: Codable, Sendable {
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucket: TodoBucket?
    public var createdAt: String?
    public var creator: Person?
    public var documentsCount: Int32?
    public var documentsUrl: String?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var position: Int32?
    public var status: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var uploadsCount: Int32?
    public var uploadsUrl: String?
    public var url: String?
    public var vaultsCount: Int32?
    public var vaultsUrl: String?
    public var visibleToClients: Bool?
}
