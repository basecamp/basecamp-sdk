// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListEntriesScheduleOptions: Sendable {
    public var status: String?
    public var maxItems: Int?

    public init(status: String? = nil, maxItems: Int? = nil) {
        self.status = status
        self.maxItems = maxItems
    }
}


public final class SchedulesService: BaseService, @unchecked Sendable {
    public func createEntry(scheduleId: Int, req: CreateScheduleEntryRequest) async throws -> ScheduleEntry {
        return try await request(
            OperationInfo(service: "Schedules", operation: "CreateScheduleEntry", resourceType: "schedule_entry", isMutation: true, resourceId: scheduleId),
            method: "POST",
            path: "/schedules/\(scheduleId)/entries.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateScheduleEntry")
        )
    }

    public func get(scheduleId: Int) async throws -> Schedule {
        return try await request(
            OperationInfo(service: "Schedules", operation: "GetSchedule", resourceType: "schedule", isMutation: false, resourceId: scheduleId),
            method: "GET",
            path: "/schedules/\(scheduleId)",
            retryConfig: Metadata.retryConfig(for: "GetSchedule")
        )
    }

    public func getEntry(entryId: Int) async throws -> ScheduleEntry {
        return try await request(
            OperationInfo(service: "Schedules", operation: "GetScheduleEntry", resourceType: "schedule_entry", isMutation: false, resourceId: entryId),
            method: "GET",
            path: "/schedule_entries/\(entryId)",
            retryConfig: Metadata.retryConfig(for: "GetScheduleEntry")
        )
    }

    public func getEntryOccurrence(entryId: Int, date: String) async throws -> ScheduleEntry {
        return try await request(
            OperationInfo(service: "Schedules", operation: "GetScheduleEntryOccurrence", resourceType: "schedule_entry_occurrence", isMutation: false, resourceId: entryId),
            method: "GET",
            path: "/schedule_entries/\(entryId)/occurrences/\(date)",
            retryConfig: Metadata.retryConfig(for: "GetScheduleEntryOccurrence")
        )
    }

    public func listEntries(scheduleId: Int, options: ListEntriesScheduleOptions? = nil) async throws -> ListResult<ScheduleEntry> {
        var queryItems: [URLQueryItem] = []
        if let status = options?.status {
            queryItems.append(URLQueryItem(name: "status", value: status))
        }
        return try await requestPaginated(
            OperationInfo(service: "Schedules", operation: "ListScheduleEntries", resourceType: "schedule_entry", isMutation: false, resourceId: scheduleId),
            path: "/schedules/\(scheduleId)/entries.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "ListScheduleEntries")
        )
    }

    public func updateEntry(entryId: Int, req: UpdateScheduleEntryRequest) async throws -> ScheduleEntry {
        return try await request(
            OperationInfo(service: "Schedules", operation: "UpdateScheduleEntry", resourceType: "schedule_entry", isMutation: true, resourceId: entryId),
            method: "PUT",
            path: "/schedule_entries/\(entryId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateScheduleEntry")
        )
    }

    public func updateSettings(scheduleId: Int, req: UpdateScheduleSettingsRequest) async throws -> Schedule {
        return try await request(
            OperationInfo(service: "Schedules", operation: "UpdateScheduleSettings", resourceType: "schedule_setting", isMutation: true, resourceId: scheduleId),
            method: "PUT",
            path: "/schedules/\(scheduleId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateScheduleSettings")
        )
    }
}
