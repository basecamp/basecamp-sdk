// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct AccountSubscription: Codable, Sendable {
    public var clients: Bool?
    public var logo: Bool?
    public var projectLimit: Int32?
    public var properName: String?
    public var shortName: String?
    public var teams: Bool?
    public var templates: Bool?
    public var timesheet: Bool?

    public init(
        clients: Bool? = nil,
        logo: Bool? = nil,
        projectLimit: Int32? = nil,
        properName: String? = nil,
        shortName: String? = nil,
        teams: Bool? = nil,
        templates: Bool? = nil,
        timesheet: Bool? = nil
    ) {
        self.clients = clients
        self.logo = logo
        self.projectLimit = projectLimit
        self.properName = properName
        self.shortName = shortName
        self.teams = teams
        self.templates = templates
        self.timesheet = timesheet
    }
}
