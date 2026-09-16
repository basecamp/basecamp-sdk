// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct Person: Codable, Sendable {
    public let id: FlexibleInt
    /// Label for system actors (e.g. "basecamp"). Present when personable_type is "LocalPerson".
    public var systemLabel: String?
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
    public var tagline: String?
    public var timeZone: String?
    public var title: String?
    public var updatedAt: String?

    public init(
        id: FlexibleInt,
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
        tagline: String? = nil,
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
        self.tagline = tagline
        self.timeZone = timeZone
        self.title = title
        self.updatedAt = updatedAt
    }

    enum CodingKeys: String, CodingKey {
        case id
        case systemLabel
        case name
        case admin
        case attachableSgid
        case avatarUrl
        case bio
        case canAccessHillCharts
        case canAccessTimesheet
        case canManagePeople
        case canManageProjects
        case canPing
        case client
        case company
        case createdAt
        case emailAddress
        case employee
        case location
        case owner
        case personableType
        case tagline
        case timeZone
        case title
        case updatedAt
    }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.id = try container.contains(.id) ? container.decode(FlexibleInt.self, forKey: .id) : FlexibleInt(0)
        self.systemLabel = try container.decodeIfPresent(String.self, forKey: .systemLabel)
        self.name = try container.decode(String.self, forKey: .name)
        self.admin = try container.decodeIfPresent(Bool.self, forKey: .admin)
        self.attachableSgid = try container.decodeIfPresent(String.self, forKey: .attachableSgid)
        self.avatarUrl = try container.decodeIfPresent(String.self, forKey: .avatarUrl)
        self.bio = try container.decodeIfPresent(String.self, forKey: .bio)
        self.canAccessHillCharts = try container.decodeIfPresent(Bool.self, forKey: .canAccessHillCharts)
        self.canAccessTimesheet = try container.decodeIfPresent(Bool.self, forKey: .canAccessTimesheet)
        self.canManagePeople = try container.decodeIfPresent(Bool.self, forKey: .canManagePeople)
        self.canManageProjects = try container.decodeIfPresent(Bool.self, forKey: .canManageProjects)
        self.canPing = try container.decodeIfPresent(Bool.self, forKey: .canPing)
        self.client = try container.decodeIfPresent(Bool.self, forKey: .client)
        self.company = try container.decodeIfPresent(PersonCompany.self, forKey: .company)
        self.createdAt = try container.decodeIfPresent(String.self, forKey: .createdAt)
        self.emailAddress = try container.decodeIfPresent(String.self, forKey: .emailAddress)
        self.employee = try container.decodeIfPresent(Bool.self, forKey: .employee)
        self.location = try container.decodeIfPresent(String.self, forKey: .location)
        self.owner = try container.decodeIfPresent(Bool.self, forKey: .owner)
        self.personableType = try container.decodeIfPresent(String.self, forKey: .personableType)
        self.tagline = try container.decodeIfPresent(String.self, forKey: .tagline)
        self.timeZone = try container.decodeIfPresent(String.self, forKey: .timeZone)
        self.title = try container.decodeIfPresent(String.self, forKey: .title)
        self.updatedAt = try container.decodeIfPresent(String.self, forKey: .updatedAt)
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(self.id, forKey: .id)
        try container.encodeIfPresent(self.systemLabel, forKey: .systemLabel)
        try container.encode(self.name, forKey: .name)
        try container.encodeIfPresent(self.admin, forKey: .admin)
        try container.encodeIfPresent(self.attachableSgid, forKey: .attachableSgid)
        try container.encodeIfPresent(self.avatarUrl, forKey: .avatarUrl)
        try container.encodeIfPresent(self.bio, forKey: .bio)
        try container.encodeIfPresent(self.canAccessHillCharts, forKey: .canAccessHillCharts)
        try container.encodeIfPresent(self.canAccessTimesheet, forKey: .canAccessTimesheet)
        try container.encodeIfPresent(self.canManagePeople, forKey: .canManagePeople)
        try container.encodeIfPresent(self.canManageProjects, forKey: .canManageProjects)
        try container.encodeIfPresent(self.canPing, forKey: .canPing)
        try container.encodeIfPresent(self.client, forKey: .client)
        try container.encodeIfPresent(self.company, forKey: .company)
        try container.encodeIfPresent(self.createdAt, forKey: .createdAt)
        try container.encodeIfPresent(self.emailAddress, forKey: .emailAddress)
        try container.encodeIfPresent(self.employee, forKey: .employee)
        try container.encodeIfPresent(self.location, forKey: .location)
        try container.encodeIfPresent(self.owner, forKey: .owner)
        try container.encodeIfPresent(self.personableType, forKey: .personableType)
        try container.encodeIfPresent(self.tagline, forKey: .tagline)
        try container.encodeIfPresent(self.timeZone, forKey: .timeZone)
        try container.encodeIfPresent(self.title, forKey: .title)
        try container.encodeIfPresent(self.updatedAt, forKey: .updatedAt)
    }
}

extension Person: ZeroPerson {
    static var zero: Person { Person(id: 0, name: "") }
}
