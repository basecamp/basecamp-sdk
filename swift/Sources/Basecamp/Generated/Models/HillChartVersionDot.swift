// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct HillChartVersionDot: Codable, Sendable {
    public let color: String
    public let label: String
    public let position: Int32
    public var appUrl: String?
    public var id: Int?
    public var url: String?

    public init(
        color: String,
        label: String,
        position: Int32,
        appUrl: String? = nil,
        id: Int? = nil,
        url: String? = nil
    ) {
        self.color = color
        self.label = label
        self.position = position
        self.appUrl = appUrl
        self.id = id
        self.url = url
    }
}
