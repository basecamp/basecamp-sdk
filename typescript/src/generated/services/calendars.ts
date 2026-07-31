/**
 * Calendars service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Calendar entity from the Basecamp API. */
export type Calendar = components["schemas"]["Calendar"];

/**
 * Request parameters for updateCalendar.
 */
export interface UpdateCalendarCalendarRequest {
  /** Calendar */
  calendar: components["schemas"]["CalendarAttributes"];
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Calendars operations.
 */
export class CalendarsService extends BaseService {

  /**
   * Get a calendar by its bucket id. A Calendar is a top-level BC5 bucketable
   * @param calendarId - The calendar ID
   * @returns The Calendar
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.calendars.getCalendar(123);
   * ```
   */
  async getCalendar(calendarId: number): Promise<Calendar> {
    const response = await this.request(
      {
        service: "Calendars",
        operation: "GetCalendar",
        resourceType: "calendar",
        isMutation: false,
        resourceId: calendarId,
      },
      () =>
        this.client.GET("/calendars/{calendarId}", {
          params: {
            path: { calendarId },
          },
        })
    );
    return response;
  }

  /**
   * Update a calendar's display color. An unknown color returns 422 with a JSON
   * @param calendarId - The calendar ID
   * @param req - Calendar update parameters
   * @returns The Calendar
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.calendars.updateCalendar(123, { calendar: { color: "example" } });
   * ```
   */
  async updateCalendar(calendarId: number, req: UpdateCalendarCalendarRequest): Promise<Calendar> {
    if (!req.calendar) {
      throw Errors.validation("Calendar is required");
    }
    const response = await this.request(
      {
        service: "Calendars",
        operation: "UpdateCalendar",
        resourceType: "calendar",
        isMutation: true,
        resourceId: calendarId,
      },
      () =>
        this.client.PUT("/calendars/{calendarId}", {
          params: {
            path: { calendarId },
          },
          body: {
            calendar: req.calendar,
          },
        })
    );
    return response;
  }
}