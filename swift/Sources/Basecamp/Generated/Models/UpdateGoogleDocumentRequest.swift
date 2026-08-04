// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct UpdateGoogleDocumentRequest: Codable, Sendable {
    public var description: String?
    public let documentType: String
    public var status: String?
    public var subscriptions: [Int]?
    public var title: String?
    public let url: String

    public init(
        description: String? = nil,
        documentType: String,
        status: String? = nil,
        subscriptions: [Int]? = nil,
        title: String? = nil,
        url: String
    ) {
        self.description = description
        self.documentType = documentType
        self.status = status
        self.subscriptions = subscriptions
        self.title = title
        self.url = url
    }
}
