// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct AccountLimits: Codable, Sendable {
    public var canCreateProjects: Bool?
    public var canCreateUsers: Bool?
    public var canPinProjects: Bool?
    public var canUploadFiles: Bool?

    public init(
        canCreateProjects: Bool? = nil,
        canCreateUsers: Bool? = nil,
        canPinProjects: Bool? = nil,
        canUploadFiles: Bool? = nil
    ) {
        self.canCreateProjects = canCreateProjects
        self.canCreateUsers = canCreateUsers
        self.canPinProjects = canPinProjects
        self.canUploadFiles = canUploadFiles
    }
}
