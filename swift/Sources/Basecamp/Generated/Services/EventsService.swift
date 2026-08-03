// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListEventOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class EventsService: BaseService, @unchecked Sendable {
    public func list(recordingId: Int, options: ListEventOptions? = nil) async throws -> ListResult<Event> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Events", operation: "ListEvents", resourceType: "event", isMutation: false, resourceId: recordingId),
            path: "/recordings/\(recordingId)/events.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListEvents")
        )
    }
}
