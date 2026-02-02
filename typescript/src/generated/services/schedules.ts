/**
 * Schedules service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** ScheduleEntry entity from the Basecamp API. */
export type ScheduleEntry = components["schemas"]["ScheduleEntry"];
/** Schedule entity from the Basecamp API. */
export type Schedule = components["schemas"]["Schedule"];

/**
 * Request parameters for updateEntry.
 */
export interface UpdateEntryScheduleRequest {
  /** Summary text */
  summary?: string;
  /** Starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  startsAt?: string;
  /** Ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  endsAt?: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Participant ids */
  participantIds?: number[];
  /** All day */
  allDay?: boolean;
  /** Whether to send notifications to relevant people */
  notify?: boolean;
}

/**
 * Request parameters for updateSettings.
 */
export interface UpdateSettingsScheduleRequest {
  /** Include due assignments */
  includeDueAssignments: boolean;
}

/**
 * Options for listEntries.
 */
export interface ListEntriesScheduleOptions extends PaginationOptions {
  /** Filter by status */
  status?: "active" | "archived" | "trashed";
}

/**
 * Request parameters for createEntry.
 */
export interface CreateEntryScheduleRequest {
  /** Summary text */
  summary: string;
  /** Starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  startsAt: string;
  /** Ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  endsAt: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Participant ids */
  participantIds?: number[];
  /** All day */
  allDay?: boolean;
  /** Whether to send notifications to relevant people */
  notify?: boolean;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Schedules operations.
 */
export class SchedulesService extends BaseService {

  /**
   * Get a single schedule entry by id.
   * @param entryId - The entry ID
   * @returns The ScheduleEntry
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.schedules.getEntry(123);
   * ```
   */
  async getEntry(entryId: number): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetScheduleEntry",
        resourceType: "schedule_entry",
        isMutation: false,
        resourceId: entryId,
      },
      () =>
        this.client.GET("/schedule_entries/{entryId}", {
          params: {
            path: { entryId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing schedule entry
   * @param entryId - The entry ID
   * @param req - Schedule_entry update parameters
   * @returns The ScheduleEntry
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.schedules.updateEntry(123, { });
   * ```
   */
  async updateEntry(entryId: number, req: UpdateEntryScheduleRequest): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "UpdateScheduleEntry",
        resourceType: "schedule_entry",
        isMutation: true,
        resourceId: entryId,
      },
      () =>
        this.client.PUT("/schedule_entries/{entryId}", {
          params: {
            path: { entryId },
          },
          body: {
            summary: req.summary,
            starts_at: req.startsAt,
            ends_at: req.endsAt,
            description: req.description,
            participant_ids: req.participantIds,
            all_day: req.allDay,
            notify: req.notify,
          },
        })
    );
    return response;
  }

  /**
   * Get a specific occurrence of a recurring schedule entry
   * @param entryId - The entry ID
   * @param date - The date
   * @returns The ScheduleEntry
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.schedules.getEntryOccurrence(123, "example");
   * ```
   */
  async getEntryOccurrence(entryId: number, date: string): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetScheduleEntryOccurrence",
        resourceType: "schedule_entry_occurrence",
        isMutation: false,
        resourceId: entryId,
      },
      () =>
        this.client.GET("/schedule_entries/{entryId}/occurrences/{date}", {
          params: {
            path: { entryId, date },
          },
        })
    );
    return response;
  }

  /**
   * Get a schedule
   * @param scheduleId - The schedule ID
   * @returns The Schedule
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.schedules.get(123);
   * ```
   */
  async get(scheduleId: number): Promise<Schedule> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetSchedule",
        resourceType: "schedule",
        isMutation: false,
        resourceId: scheduleId,
      },
      () =>
        this.client.GET("/schedules/{scheduleId}", {
          params: {
            path: { scheduleId },
          },
        })
    );
    return response;
  }

  /**
   * Update schedule settings
   * @param scheduleId - The schedule ID
   * @param req - Schedule_setting update parameters
   * @returns The Schedule
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.schedules.updateSettings(123, { includeDueAssignments: true });
   * ```
   */
  async updateSettings(scheduleId: number, req: UpdateSettingsScheduleRequest): Promise<Schedule> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "UpdateScheduleSettings",
        resourceType: "schedule_setting",
        isMutation: true,
        resourceId: scheduleId,
      },
      () =>
        this.client.PUT("/schedules/{scheduleId}", {
          params: {
            path: { scheduleId },
          },
          body: {
            include_due_assignments: req.includeDueAssignments,
          },
        })
    );
    return response;
  }

  /**
   * List entries on a schedule
   * @param scheduleId - The schedule ID
   * @param options - Optional query parameters
   * @returns All ScheduleEntry across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.schedules.listEntries(123);
   *
   * // With options
   * const filtered = await client.schedules.listEntries(123, { status: "active" });
   * ```
   */
  async listEntries(scheduleId: number, options?: ListEntriesScheduleOptions): Promise<ListResult<ScheduleEntry>> {
    return this.requestPaginated(
      {
        service: "Schedules",
        operation: "ListScheduleEntries",
        resourceType: "schedule_entrie",
        isMutation: false,
        resourceId: scheduleId,
      },
      () =>
        this.client.GET("/schedules/{scheduleId}/entries.json", {
          params: {
            path: { scheduleId },
            query: { status: options?.status },
          },
        })
      , options
    );
  }

  /**
   * Create a new schedule entry
   * @param scheduleId - The schedule ID
   * @param req - Schedule_entry creation parameters
   * @returns The ScheduleEntry
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.schedules.createEntry(123, { summary: "example", startsAt: "2025-06-01T09:00:00Z", endsAt: "2025-06-01T09:00:00Z" });
   * ```
   */
  async createEntry(scheduleId: number, req: CreateEntryScheduleRequest): Promise<ScheduleEntry> {
    if (!req.summary) {
      throw Errors.validation("Summary is required");
    }
    if (!req.startsAt) {
      throw Errors.validation("Starts at is required");
    }
    if (!req.endsAt) {
      throw Errors.validation("Ends at is required");
    }
    const response = await this.request(
      {
        service: "Schedules",
        operation: "CreateScheduleEntry",
        resourceType: "schedule_entry",
        isMutation: true,
        resourceId: scheduleId,
      },
      () =>
        this.client.POST("/schedules/{scheduleId}/entries.json", {
          params: {
            path: { scheduleId },
          },
          body: {
            summary: req.summary,
            starts_at: req.startsAt,
            ends_at: req.endsAt,
            description: req.description,
            participant_ids: req.participantIds,
            all_day: req.allDay,
            notify: req.notify,
          },
        })
    );
    return response;
  }
}