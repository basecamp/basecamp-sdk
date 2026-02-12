// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ClientApproval: Codable, Sendable {
    public var appUrl: String?
    public var approvalStatus: String?
    public var approver: Person?
    public var bookmarkUrl: String?
    public var bucket: RecordingBucket?
    public var content: String?
    public var createdAt: String?
    public var creator: Person?
    public var dueOn: String?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var repliesCount: Int32?
    public var repliesUrl: String?
    public var responses: [ClientApprovalResponse]?
    public var status: String?
    public var subject: String?
    public var subscriptionUrl: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
