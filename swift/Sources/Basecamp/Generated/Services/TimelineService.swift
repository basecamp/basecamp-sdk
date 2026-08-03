// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ProjectTimelineTimelineOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class TimelineService: BaseService, @unchecked Sendable {
    public func projectTimeline(projectId: Int, options: ProjectTimelineTimelineOptions? = nil) async throws -> ListResult<TimelineEvent> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Timeline", operation: "GetProjectTimeline", resourceType: "project_timeline", isMutation: false, projectId: projectId),
            path: "/projects/\(projectId)/timeline.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "GetProjectTimeline")
        )
    }
}
