// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListEventOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class EventsService: BaseService, @unchecked Sendable {
    public func list(recordingId: Int, options: ListEventOptions? = nil) async throws -> ListResult<Event> {
        return try await requestPaginated(
            OperationInfo(service: "Events", operation: "ListEvents", resourceType: "event", isMutation: false, resourceId: recordingId),
            path: "/recordings/\(recordingId)/events.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListEvents")
        )
    }
}
