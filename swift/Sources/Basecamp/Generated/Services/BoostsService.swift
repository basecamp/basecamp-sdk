// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListForEventBoostOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

public struct ListForRecordingBoostOptions: Sendable {
    public var maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}


public final class BoostsService: BaseService, @unchecked Sendable {
    public func createForEvent(projectId: Int, recordingId: Int, eventId: Int, req: CreateEventBoostRequest) async throws -> Boost {
        return try await request(
            OperationInfo(service: "Boosts", operation: "CreateEventBoost", resourceType: "event_boost", isMutation: true, projectId: projectId, resourceId: recordingId),
            method: "POST",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/events/\(eventId)/boosts.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateEventBoost")
        )
    }

    public func createForRecording(projectId: Int, recordingId: Int, req: CreateRecordingBoostRequest) async throws -> Boost {
        return try await request(
            OperationInfo(service: "Boosts", operation: "CreateRecordingBoost", resourceType: "recording_boost", isMutation: true, projectId: projectId, resourceId: recordingId),
            method: "POST",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/boosts.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateRecordingBoost")
        )
    }

    public func delete(projectId: Int, boostId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Boosts", operation: "DeleteBoost", resourceType: "boost", isMutation: true, projectId: projectId, resourceId: boostId),
            method: "DELETE",
            path: "/buckets/\(projectId)/boosts/\(boostId)",
            retryConfig: Metadata.retryConfig(for: "DeleteBoost")
        )
    }

    public func get(projectId: Int, boostId: Int) async throws -> Boost {
        return try await request(
            OperationInfo(service: "Boosts", operation: "GetBoost", resourceType: "boost", isMutation: false, projectId: projectId, resourceId: boostId),
            method: "GET",
            path: "/buckets/\(projectId)/boosts/\(boostId)",
            retryConfig: Metadata.retryConfig(for: "GetBoost")
        )
    }

    public func listForEvent(projectId: Int, recordingId: Int, eventId: Int, options: ListForEventBoostOptions? = nil) async throws -> ListResult<Boost> {
        return try await requestPaginated(
            OperationInfo(service: "Boosts", operation: "ListEventBoosts", resourceType: "event_boost", isMutation: false, projectId: projectId, resourceId: recordingId),
            path: "/buckets/\(projectId)/recordings/\(recordingId)/events/\(eventId)/boosts.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListEventBoosts")
        )
    }

    public func listForRecording(projectId: Int, recordingId: Int, options: ListForRecordingBoostOptions? = nil) async throws -> ListResult<Boost> {
        return try await requestPaginated(
            OperationInfo(service: "Boosts", operation: "ListRecordingBoosts", resourceType: "recording_boost", isMutation: false, projectId: projectId, resourceId: recordingId),
            path: "/buckets/\(projectId)/recordings/\(recordingId)/boosts.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListRecordingBoosts")
        )
    }
}
