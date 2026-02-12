// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Upload: Codable, Sendable {
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var boostsCount: Int32?
    public var boostsUrl: String?
    public var bucket: TodoBucket?
    public var byteSize: Int?
    public var commentsCount: Int32?
    public var commentsUrl: String?
    public var contentType: String?
    public var createdAt: String?
    public var creator: Person?
    public var description: String?
    public var downloadUrl: String?
    public var filename: String?
    public var height: Int32?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var position: Int32?
    public var status: String?
    public var subscriptionUrl: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
    public var width: Int32?
}
