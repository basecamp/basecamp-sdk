/**
 * Schedules service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

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
  /** summary */
  summary?: string;
  /** starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  startsAt?: string;
  /** ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  endsAt?: string;
  /** description */
  description?: string;
  /** participant ids */
  participantIds?: number[];
  /** all day */
  allDay?: boolean;
  /** notify */
  notify?: boolean;
}

/**
 * Request parameters for updateSettings.
 */
export interface UpdateSettingsScheduleRequest {
  /** include due assignments */
  includeDueAssignments: boolean;
}

/**
 * Options for listEntries.
 */
export interface ListEntriesScheduleOptions {
  /** active|archived|trashed */
  status?: string;
}

/**
 * Request parameters for createEntry.
 */
export interface CreateEntryScheduleRequest {
  /** summary */
  summary: string;
  /** starts at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  startsAt: string;
  /** ends at (RFC3339 (e.g., 2024-12-15T09:00:00Z)) */
  endsAt: string;
  /** description */
  description?: string;
  /** participant ids */
  participantIds?: number[];
  /** all day */
  allDay?: boolean;
  /** notify */
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
   * Get a single schedule entry by id
   * @param projectId - The project ID
   * @param entryId - The entry ID
   * @returns The ScheduleEntry
   */
  async getEntry(projectId: number, entryId: number): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetScheduleEntry",
        resourceType: "schedule_entry",
        isMutation: false,
        projectId,
        resourceId: entryId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/schedule_entries/{entryId}", {
          params: {
            path: { projectId, entryId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing schedule entry
   * @param projectId - The project ID
   * @param entryId - The entry ID
   * @param req - Request parameters
   * @returns The ScheduleEntry
   */
  async updateEntry(projectId: number, entryId: number, req: UpdateEntryScheduleRequest): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "UpdateScheduleEntry",
        resourceType: "schedule_entry",
        isMutation: true,
        projectId,
        resourceId: entryId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/schedule_entries/{entryId}", {
          params: {
            path: { projectId, entryId },
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
   * @param projectId - The project ID
   * @param entryId - The entry ID
   * @param date - The date
   * @returns The ScheduleEntry
   */
  async getEntryOccurrence(projectId: number, entryId: number, date: string): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetScheduleEntryOccurrence",
        resourceType: "schedule_entry_occurrence",
        isMutation: false,
        projectId,
        resourceId: entryId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/schedule_entries/{entryId}/occurrences/{date}", {
          params: {
            path: { projectId, entryId, date },
          },
        })
    );
    return response;
  }

  /**
   * Get a schedule
   * @param projectId - The project ID
   * @param scheduleId - The schedule ID
   * @returns The Schedule
   */
  async get(projectId: number, scheduleId: number): Promise<Schedule> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetSchedule",
        resourceType: "schedule",
        isMutation: false,
        projectId,
        resourceId: scheduleId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/schedules/{scheduleId}", {
          params: {
            path: { projectId, scheduleId },
          },
        })
    );
    return response;
  }

  /**
   * Update schedule settings
   * @param projectId - The project ID
   * @param scheduleId - The schedule ID
   * @param req - Request parameters
   * @returns The Schedule
   */
  async updateSettings(projectId: number, scheduleId: number, req: UpdateSettingsScheduleRequest): Promise<Schedule> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "UpdateScheduleSettings",
        resourceType: "schedule_setting",
        isMutation: true,
        projectId,
        resourceId: scheduleId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/schedules/{scheduleId}", {
          params: {
            path: { projectId, scheduleId },
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
   * @param projectId - The project ID
   * @param scheduleId - The schedule ID
   * @param options - Optional parameters
   * @returns Array of ScheduleEntry
   */
  async listEntries(projectId: number, scheduleId: number, options?: ListEntriesScheduleOptions): Promise<ScheduleEntry[]> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "ListScheduleEntries",
        resourceType: "schedule_entrie",
        isMutation: false,
        projectId,
        resourceId: scheduleId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/schedules/{scheduleId}/entries.json", {
          params: {
            path: { projectId, scheduleId },
            query: { status: options?.status },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new schedule entry
   * @param projectId - The project ID
   * @param scheduleId - The schedule ID
   * @param req - Request parameters
   * @returns The ScheduleEntry
   *
   * @example
   * ```ts
   * const result = await client.schedules.createEntry(123, 123, { ... });
   * ```
   */
  async createEntry(projectId: number, scheduleId: number, req: CreateEntryScheduleRequest): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "CreateScheduleEntry",
        resourceType: "schedule_entry",
        isMutation: true,
        projectId,
        resourceId: scheduleId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/schedules/{scheduleId}/entries.json", {
          params: {
            path: { projectId, scheduleId },
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