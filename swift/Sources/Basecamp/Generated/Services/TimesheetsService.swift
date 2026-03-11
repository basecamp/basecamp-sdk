// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ForProjectTimesheetOptions: Sendable {
    public var from: String?
    public var to: String?
    public var personId: Int?
    public var maxItems: Int?

    public init(
        from: String? = nil,
        to: String? = nil,
        personId: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.from = from
        self.to = to
        self.personId = personId
        self.maxItems = maxItems
    }
}

public struct ForRecordingTimesheetOptions: Sendable {
    public var from: String?
    public var to: String?
    public var personId: Int?
    public var maxItems: Int?

    public init(
        from: String? = nil,
        to: String? = nil,
        personId: Int? = nil,
        maxItems: Int? = nil
    ) {
        self.from = from
        self.to = to
        self.personId = personId
        self.maxItems = maxItems
    }
}

public struct ReportTimesheetOptions: Sendable {
    public var from: String?
    public var to: String?
    public var personId: Int?

    public init(from: String? = nil, to: String? = nil, personId: Int? = nil) {
        self.from = from
        self.to = to
        self.personId = personId
    }
}


public final class TimesheetsService: BaseService, @unchecked Sendable {
    public func create(recordingId: Int, req: CreateTimesheetEntryRequest) async throws -> TimesheetEntry {
        return try await request(
            OperationInfo(service: "Timesheets", operation: "CreateTimesheetEntry", resourceType: "timesheet_entry", isMutation: true, resourceId: recordingId),
            method: "POST",
            path: "/recordings/\(recordingId)/timesheet/entries.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateTimesheetEntry")
        )
    }

    public func forProject(projectId: Int, options: ForProjectTimesheetOptions? = nil) async throws -> ListResult<TimesheetEntry> {
        var queryItems: [URLQueryItem] = []
        if let from = options?.from {
            queryItems.append(URLQueryItem(name: "from", value: from))
        }
        if let to = options?.to {
            queryItems.append(URLQueryItem(name: "to", value: to))
        }
        if let personId = options?.personId {
            queryItems.append(URLQueryItem(name: "person_id", value: String(personId)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Timesheets", operation: "GetProjectTimesheet", resourceType: "project_timesheet", isMutation: false, projectId: projectId),
            path: "/projects/\(projectId)/timesheet.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetProjectTimesheet")
        )
    }

    public func forRecording(recordingId: Int, options: ForRecordingTimesheetOptions? = nil) async throws -> ListResult<TimesheetEntry> {
        var queryItems: [URLQueryItem] = []
        if let from = options?.from {
            queryItems.append(URLQueryItem(name: "from", value: from))
        }
        if let to = options?.to {
            queryItems.append(URLQueryItem(name: "to", value: to))
        }
        if let personId = options?.personId {
            queryItems.append(URLQueryItem(name: "person_id", value: String(personId)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Timesheets", operation: "GetRecordingTimesheet", resourceType: "recording_timesheet", isMutation: false, resourceId: recordingId),
            path: "/recordings/\(recordingId)/timesheet.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems) },
            retryConfig: Metadata.retryConfig(for: "GetRecordingTimesheet")
        )
    }

    public func get(entryId: Int) async throws -> TimesheetEntry {
        return try await request(
            OperationInfo(service: "Timesheets", operation: "GetTimesheetEntry", resourceType: "timesheet_entry", isMutation: false, resourceId: entryId),
            method: "GET",
            path: "/timesheet_entries/\(entryId)",
            retryConfig: Metadata.retryConfig(for: "GetTimesheetEntry")
        )
    }

    public func report(options: ReportTimesheetOptions? = nil) async throws -> [TimesheetEntry] {
        var queryItems: [URLQueryItem] = []
        if let from = options?.from {
            queryItems.append(URLQueryItem(name: "from", value: from))
        }
        if let to = options?.to {
            queryItems.append(URLQueryItem(name: "to", value: to))
        }
        if let personId = options?.personId {
            queryItems.append(URLQueryItem(name: "person_id", value: String(personId)))
        }
        return try await request(
            OperationInfo(service: "Timesheets", operation: "GetTimesheetReport", resourceType: "timesheet_report", isMutation: false),
            method: "GET",
            path: "/reports/timesheet.json" + queryString(queryItems),
            retryConfig: Metadata.retryConfig(for: "GetTimesheetReport")
        )
    }

    public func update(entryId: Int, req: UpdateTimesheetEntryRequest) async throws -> TimesheetEntry {
        return try await request(
            OperationInfo(service: "Timesheets", operation: "UpdateTimesheetEntry", resourceType: "timesheet_entry", isMutation: true, resourceId: entryId),
            method: "PUT",
            path: "/timesheet_entries/\(entryId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateTimesheetEntry")
        )
    }
}
