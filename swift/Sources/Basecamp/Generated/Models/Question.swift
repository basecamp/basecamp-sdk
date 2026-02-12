// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Question: Codable, Sendable {
    public var answersCount: Int32?
    public var answersUrl: String?
    public var appUrl: String?
    public var bookmarkUrl: String?
    public var bucket: RecordingBucket?
    public var createdAt: String?
    public var creator: Person?
    public var id: Int?
    public var inheritsStatus: Bool?
    public var parent: RecordingParent?
    public var paused: Bool?
    public var schedule: QuestionSchedule?
    public var status: String?
    public var subscriptionUrl: String?
    public var title: String?
    public var type: String?
    public var updatedAt: String?
    public var url: String?
    public var visibleToClients: Bool?
}
