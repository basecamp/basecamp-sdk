// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct CreateProjectFromTemplateRequest: Codable, Sendable {
    public let project: ProjectConstructionAttributes

    public init(project: ProjectConstructionAttributes) {
        self.project = project
    }
}
