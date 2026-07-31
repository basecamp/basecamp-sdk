// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class CalendarsService: BaseService, @unchecked Sendable {
    public func getCalendar(calendarId: Int) async throws -> Calendar {
        return try await request(
            OperationInfo(service: "Calendars", operation: "GetCalendar", resourceType: "calendar", isMutation: false, resourceId: calendarId),
            method: "GET",
            path: "/calendars/\(calendarId)",
            retryConfig: Metadata.retryConfig(for: "GetCalendar")
        )
    }

    public func updateCalendar(calendarId: Int, req: UpdateCalendarRequest) async throws -> Calendar {
        return try await request(
            OperationInfo(service: "Calendars", operation: "UpdateCalendar", resourceType: "calendar", isMutation: true, resourceId: calendarId),
            method: "PUT",
            path: "/calendars/\(calendarId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateCalendar")
        )
    }
}
