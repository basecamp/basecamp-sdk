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
    public func createForEvent(recordingId: Int, eventId: Int, req: CreateEventBoostRequest) async throws -> Boost {
        return try await request(
            OperationInfo(service: "Boosts", operation: "CreateEventBoost", resourceType: "event_boost", isMutation: true, resourceId: recordingId),
            method: "POST",
            path: "/recordings/\(recordingId)/events/\(eventId)/boosts.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateEventBoost")
        )
    }

    public func createForRecording(recordingId: Int, req: CreateRecordingBoostRequest) async throws -> Boost {
        return try await request(
            OperationInfo(service: "Boosts", operation: "CreateRecordingBoost", resourceType: "recording_boost", isMutation: true, resourceId: recordingId),
            method: "POST",
            path: "/recordings/\(recordingId)/boosts.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateRecordingBoost")
        )
    }

    public func delete(boostId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Boosts", operation: "DeleteBoost", resourceType: "boost", isMutation: true, resourceId: boostId),
            method: "DELETE",
            path: "/boosts/\(boostId)",
            retryConfig: Metadata.retryConfig(for: "DeleteBoost")
        )
    }

    public func get(boostId: Int) async throws -> Boost {
        return try await request(
            OperationInfo(service: "Boosts", operation: "GetBoost", resourceType: "boost", isMutation: false, resourceId: boostId),
            method: "GET",
            path: "/boosts/\(boostId)",
            retryConfig: Metadata.retryConfig(for: "GetBoost")
        )
    }

    public func listForEvent(recordingId: Int, eventId: Int, options: ListForEventBoostOptions? = nil) async throws -> ListResult<Boost> {
        return try await requestPaginated(
            OperationInfo(service: "Boosts", operation: "ListEventBoosts", resourceType: "event_boost", isMutation: false, resourceId: recordingId),
            path: "/recordings/\(recordingId)/events/\(eventId)/boosts.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListEventBoosts")
        )
    }

    public func listForRecording(recordingId: Int, options: ListForRecordingBoostOptions? = nil) async throws -> ListResult<Boost> {
        return try await requestPaginated(
            OperationInfo(service: "Boosts", operation: "ListRecordingBoosts", resourceType: "recording_boost", isMutation: false, resourceId: recordingId),
            path: "/recordings/\(recordingId)/boosts.json",
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListRecordingBoosts")
        )
    }
}
