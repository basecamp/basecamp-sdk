// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListRecordingOptions: Sendable {
    public var bucket: String?
    public var status: String?
    public var sort: String?
    public var direction: String?
    public var maxItems: Int?

    public init(
        bucket: String? = nil,
        status: String? = nil,
        sort: String? = nil,
        direction: String? = nil,
        maxItems: Int? = nil
    ) {
        self.bucket = bucket
        self.status = status
        self.sort = sort
        self.direction = direction
        self.maxItems = maxItems
    }
}


public final class RecordingsService: BaseService, @unchecked Sendable {
    public func archive(projectId: Int, recordingId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Recordings", operation: "ArchiveRecording", resourceType: "recording", isMutation: true, projectId: projectId, resourceId: recordingId),
            method: "PUT",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/status/archived.json",
            retryConfig: Metadata.retryConfig(for: "ArchiveRecording")
        )
    }

    public func get(projectId: Int, recordingId: Int) async throws -> Recording {
        return try await request(
            OperationInfo(service: "Recordings", operation: "GetRecording", resourceType: "recording", isMutation: false, projectId: projectId, resourceId: recordingId),
            method: "GET",
            path: "/buckets/\(projectId)/recordings/\(recordingId)",
            retryConfig: Metadata.retryConfig(for: "GetRecording")
        )
    }

    public func list(type: String, options: ListRecordingOptions? = nil) async throws -> ListResult<Recording> {
        var queryItems: [URLQueryItem] = []
        queryItems.append(URLQueryItem(name: "type", value: type))
        if let bucket = options?.bucket {
            queryItems.append(URLQueryItem(name: "bucket", value: bucket))
        }
        if let status = options?.status {
            queryItems.append(URLQueryItem(name: "status", value: status))
        }
        if let sort = options?.sort {
            queryItems.append(URLQueryItem(name: "sort", value: sort))
        }
        if let direction = options?.direction {
            queryItems.append(URLQueryItem(name: "direction", value: direction))
        }
        return try await requestPaginated(
            OperationInfo(service: "Recordings", operation: "ListRecordings", resourceType: "recording", isMutation: false),
            path: "/projects/recordings.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListRecordings")
        )
    }

    public func trash(projectId: Int, recordingId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Recordings", operation: "TrashRecording", resourceType: "recording", isMutation: true, projectId: projectId, resourceId: recordingId),
            method: "PUT",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/status/trashed.json",
            retryConfig: Metadata.retryConfig(for: "TrashRecording")
        )
    }

    public func unarchive(projectId: Int, recordingId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Recordings", operation: "UnarchiveRecording", resourceType: "recording", isMutation: true, projectId: projectId, resourceId: recordingId),
            method: "PUT",
            path: "/buckets/\(projectId)/recordings/\(recordingId)/status/active.json",
            retryConfig: Metadata.retryConfig(for: "UnarchiveRecording")
        )
    }
}
