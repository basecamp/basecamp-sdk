// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct AccountSettings: Codable, Sendable {
    public var companyHqEnabled: Bool?
    public var projectsEnabled: Bool?
    public var teamsEnabled: Bool?

    public init(companyHqEnabled: Bool? = nil, projectsEnabled: Bool? = nil, teamsEnabled: Bool? = nil) {
        self.companyHqEnabled = companyHqEnabled
        self.projectsEnabled = projectsEnabled
        self.teamsEnabled = teamsEnabled
    }
}
