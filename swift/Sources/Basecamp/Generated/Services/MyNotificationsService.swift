// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct MyNotificationsMyNotificationOptions: Sendable {
    public var page: Int?

    public init(page: Int? = nil) {
        self.page = page
    }
}


public final class MyNotificationsService: BaseService, @unchecked Sendable {
    public func myNotifications(options: MyNotificationsMyNotificationOptions? = nil) async throws -> GetMyNotificationsResponseContent {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await request(
            OperationInfo(service: "MyNotifications", operation: "GetMyNotifications", resourceType: "my_notification", isMutation: false),
            method: "GET",
            path: "/my/readings.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetMyNotifications")
        )
    }

    public func markAsRead(req: MarkAsReadRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "MyNotifications", operation: "MarkAsRead", resourceType: "resource", isMutation: true),
            method: "PUT",
            path: "/my/unreads.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "MarkAsRead")
        )
    }
}
