// @generated from OpenAPI spec — do not edit directly
import Foundation

/// Deprecated: This shape is deprecated since 2024-01: Use Client Visibility feature instead
public struct ClientSide: Codable, Sendable {
    public var appUrl: String?
    public var url: String?

    public init(appUrl: String? = nil, url: String? = nil) {
        self.appUrl = appUrl
        self.url = url
    }
}
