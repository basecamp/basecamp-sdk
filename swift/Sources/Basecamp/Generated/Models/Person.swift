// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Person: Codable, Sendable {
    public var admin: Bool?
    public var attachableSgid: String?
    public var avatarUrl: String?
    public var bio: String?
    public var canAccessHillCharts: Bool?
    public var canAccessTimesheet: Bool?
    public var canManagePeople: Bool?
    public var canManageProjects: Bool?
    public var canPing: Bool?
    public var client: Bool?
    public var company: PersonCompany?
    public var createdAt: String?
    public var emailAddress: String?
    public var employee: Bool?
    public var id: Int?
    public var location: String?
    public var name: String?
    public var owner: Bool?
    public var personableType: String?
    public var timeZone: String?
    public var title: String?
    public var updatedAt: String?
}
