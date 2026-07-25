// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct BubbleUpsMyNotificationOptions: Sendable {
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct MyNotificationsMyNotificationOptions: Sendable {
    public var page: Int?
    public var limitBubbleUps: Bool?

    public init(page: Int? = nil, limitBubbleUps: Bool? = nil) {
        self.page = page
        self.limitBubbleUps = limitBubbleUps
    }
}


public final class MyNotificationsService: BaseService, @unchecked Sendable {
    public func bubbleUps(options: BubbleUpsMyNotificationOptions? = nil) async throws -> ListResult<Notification> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "MyNotifications", operation: "GetBubbleUps", resourceType: "bubble_up", isMutation: false),
            path: "/my/readings/bubble_ups.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetBubbleUps")
        )
    }

    public func myNotifications(options: MyNotificationsMyNotificationOptions? = nil) async throws -> GetMyNotificationsResponseContent {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        if let limitBubbleUps = options?.limitBubbleUps {
            queryItems.append(URLQueryItem(name: "limit_bubble_ups", value: String(limitBubbleUps)))
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
