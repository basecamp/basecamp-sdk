// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Person: Codable, Sendable {
    public let id: Int
    public let name: String
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
    public var location: String?
    public var owner: Bool?
    public var personableType: String?
    public var timeZone: String?
    public var title: String?
    public var updatedAt: String?

    public init(
        id: Int,
        name: String,
        admin: Bool? = nil,
        attachableSgid: String? = nil,
        avatarUrl: String? = nil,
        bio: String? = nil,
        canAccessHillCharts: Bool? = nil,
        canAccessTimesheet: Bool? = nil,
        canManagePeople: Bool? = nil,
        canManageProjects: Bool? = nil,
        canPing: Bool? = nil,
        client: Bool? = nil,
        company: PersonCompany? = nil,
        createdAt: String? = nil,
        emailAddress: String? = nil,
        employee: Bool? = nil,
        location: String? = nil,
        owner: Bool? = nil,
        personableType: String? = nil,
        timeZone: String? = nil,
        title: String? = nil,
        updatedAt: String? = nil
    ) {
        self.id = id
        self.name = name
        self.admin = admin
        self.attachableSgid = attachableSgid
        self.avatarUrl = avatarUrl
        self.bio = bio
        self.canAccessHillCharts = canAccessHillCharts
        self.canAccessTimesheet = canAccessTimesheet
        self.canManagePeople = canManagePeople
        self.canManageProjects = canManageProjects
        self.canPing = canPing
        self.client = client
        self.company = company
        self.createdAt = createdAt
        self.emailAddress = emailAddress
        self.employee = employee
        self.location = location
        self.owner = owner
        self.personableType = personableType
        self.timeZone = timeZone
        self.title = title
        self.updatedAt = updatedAt
    }
}
