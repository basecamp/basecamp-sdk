// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ProjectTimelineTimelineOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class TimelineService: BaseService, @unchecked Sendable {
    public func projectTimeline(projectId: Int, options: ProjectTimelineTimelineOptions? = nil) async throws -> ListResult<TimelineEvent> {
        return try await requestPaginated(
            OperationInfo(service: "Timeline", operation: "GetProjectTimeline", resourceType: "project_timeline", isMutation: false, projectId: projectId),
            path: "/projects/\(projectId)/timeline.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetProjectTimeline")
        )
    }
}
