// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct BubbleUpsMyNotificationOptions: Sendable {
    /// Page number. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct MyNotificationsMyNotificationOptions: Sendable {
    /// Page number for paginating through read items. Defaults to 1. This operation is not auto-paginated in any SDK, so a page is returned as asked for and later pages are not followed.
    public var page: Int?
    /// Set to true to cap `bubble_ups` at 2 current bubble-ups and omit the `scheduled_bubble_ups` key entirely. Defaults to false. Use the dedicated bubble-ups endpoint (GetBubbleUps) to page through all current and scheduled bubble-ups.
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
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
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
