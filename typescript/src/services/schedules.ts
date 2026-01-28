/**
 * Schedules service for the Basecamp API.
 *
 * Schedules are calendars within projects that contain schedule entries (events).
 * Each project has one schedule that can optionally show todo due dates.
 *
 * @example
 * ```ts
 * // Get a schedule
 * const schedule = await client.schedules.get(projectId, scheduleId);
 *
 * // List entries on a schedule
 * const entries = await client.schedules.listEntries(projectId, scheduleId);
 *
 * // Create a new entry
 * const entry = await client.schedules.createEntry(projectId, scheduleId, {
 *   summary: "Team Meeting",
 *   startsAt: "2024-12-15T09:00:00Z",
 *   endsAt: "2024-12-15T10:00:00Z",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp schedule (calendar).
 */
export type Schedule = components["schemas"]["Schedule"];

/**
 * A schedule entry (event).
 */
export type ScheduleEntry = components["schemas"]["ScheduleEntry"];

/**
 * A person associated with schedules/entries (creator, participant).
 */
export type Person = components["schemas"]["Person"];

/**
 * Request to create a new schedule entry.
 */
export interface CreateScheduleEntryRequest {
  /** Event title (required) */
  summary: string;
  /** Event start time in RFC3339 format (required) */
  startsAt: string;
  /** Event end time in RFC3339 format (required) */
  endsAt: string;
  /** Event description in HTML (optional) */
  description?: string;
  /** Person IDs to assign as participants (optional) */
  participantIds?: number[];
  /** Whether this is an all-day event (optional) */
  allDay?: boolean;
  /** Notify participants when true (optional) */
  notify?: boolean;
}

/**
 * Request to update an existing schedule entry.
 */
export interface UpdateScheduleEntryRequest {
  /** Event title (optional) */
  summary?: string;
  /** Event start time in RFC3339 format (optional) */
  startsAt?: string;
  /** Event end time in RFC3339 format (optional) */
  endsAt?: string;
  /** Event description in HTML (optional) */
  description?: string;
  /** Person IDs to assign as participants (optional) */
  participantIds?: number[];
  /** Whether this is an all-day event (optional) */
  allDay?: boolean;
  /** Notify participants when true (optional) */
  notify?: boolean;
}

/**
 * Request to update schedule settings.
 */
export interface UpdateScheduleSettingsRequest {
  /** Whether to show todo due dates on the schedule */
  includeDueAssignments: boolean;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp schedules and entries.
 */
export class SchedulesService extends BaseService {
  /**
   * Gets a schedule by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param scheduleId - The schedule ID
   * @returns The schedule
   * @throws BasecampError with code "not_found" if schedule doesn't exist
   *
   * @example
   * ```ts
   * const schedule = await client.schedules.get(projectId, scheduleId);
   * console.log(schedule.title, schedule.entries_count);
   * ```
   */
  async get(projectId: number, scheduleId: number): Promise<Schedule> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "Get",
        resourceType: "schedule",
        isMutation: false,
        projectId,
        resourceId: scheduleId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/schedules/{scheduleId}", {
          params: { path: { projectId, scheduleId } },
        })
    );

    return response.schedule!;
  }

  /**
   * Lists all entries on a schedule.
   *
   * @param projectId - The project (bucket) ID
   * @param scheduleId - The schedule ID
   * @returns Array of schedule entries
   *
   * @example
   * ```ts
   * const entries = await client.schedules.listEntries(projectId, scheduleId);
   * for (const entry of entries) {
   *   console.log(entry.summary, entry.starts_at, entry.ends_at);
   * }
   * ```
   */
  async listEntries(projectId: number, scheduleId: number): Promise<ScheduleEntry[]> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "ListEntries",
        resourceType: "schedule_entry",
        isMutation: false,
        projectId,
        resourceId: scheduleId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/schedules/{scheduleId}/entries.json", {
          params: { path: { projectId, scheduleId } },
        })
    );

    return response?.entries ?? [];
  }

  /**
   * Gets a schedule entry by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param entryId - The schedule entry ID
   * @returns The schedule entry
   * @throws BasecampError with code "not_found" if entry doesn't exist
   *
   * @example
   * ```ts
   * const entry = await client.schedules.getEntry(projectId, entryId);
   * console.log(entry.summary, entry.all_day, entry.participants);
   * ```
   */
  async getEntry(projectId: number, entryId: number): Promise<ScheduleEntry> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetEntry",
        resourceType: "schedule_entry",
        isMutation: false,
        projectId,
        resourceId: entryId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/schedule_entries/{entryId}", {
          params: { path: { projectId, entryId } },
        })
    );

    return response.entry!;
  }

  /**
   * Creates a new entry on a schedule.
   *
   * @param projectId - The project (bucket) ID
   * @param scheduleId - The schedule ID
   * @param req - Entry creation parameters
   * @returns The created schedule entry
   * @throws BasecampError with code "validation" if required fields are missing
   *
   * @example
   * ```ts
   * const entry = await client.schedules.createEntry(projectId, scheduleId, {
   *   summary: "Project Kickoff",
   *   startsAt: "2024-12-15T14:00:00Z",
   *   endsAt: "2024-12-15T15:30:00Z",
   *   description: "<p>Kickoff meeting for new project</p>",
   *   participantIds: [1001, 1002, 1003],
   * });
   * ```
   */
  async createEntry(
    projectId: number,
    scheduleId: number,
    req: CreateScheduleEntryRequest
  ): Promise<ScheduleEntry> {
    if (!req.summary) {
      throw Errors.validation("Schedule entry summary is required");
    }
    if (!req.startsAt) {
      throw Errors.validation("Schedule entry starts_at is required");
    }
    if (!req.endsAt) {
      throw Errors.validation("Schedule entry ends_at is required");
    }

    // Validate date formats
    if (!isValidRFC3339(req.startsAt)) {
      throw Errors.validation(
        "Schedule entry starts_at must be in RFC3339 format (e.g., 2024-01-15T09:00:00Z)"
      );
    }
    if (!isValidRFC3339(req.endsAt)) {
      throw Errors.validation(
        "Schedule entry ends_at must be in RFC3339 format (e.g., 2024-01-15T17:00:00Z)"
      );
    }

    const response = await this.request(
      {
        service: "Schedules",
        operation: "CreateEntry",
        resourceType: "schedule_entry",
        isMutation: true,
        projectId,
        resourceId: scheduleId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/schedules/{scheduleId}/entries.json", {
          params: { path: { projectId, scheduleId } },
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

    return response.entry!;
  }

  /**
   * Updates an existing schedule entry.
   *
   * @param projectId - The project (bucket) ID
   * @param entryId - The schedule entry ID
   * @param req - Entry update parameters
   * @returns The updated schedule entry
   *
   * @example
   * ```ts
   * const updated = await client.schedules.updateEntry(projectId, entryId, {
   *   summary: "Updated Meeting Title",
   *   startsAt: "2024-12-15T10:00:00Z",
   *   endsAt: "2024-12-15T11:00:00Z",
   * });
   * ```
   */
  async updateEntry(
    projectId: number,
    entryId: number,
    req: UpdateScheduleEntryRequest
  ): Promise<ScheduleEntry> {
    // Validate date formats if provided
    if (req.startsAt && !isValidRFC3339(req.startsAt)) {
      throw Errors.validation(
        "Schedule entry starts_at must be in RFC3339 format (e.g., 2024-01-15T09:00:00Z)"
      );
    }
    if (req.endsAt && !isValidRFC3339(req.endsAt)) {
      throw Errors.validation(
        "Schedule entry ends_at must be in RFC3339 format (e.g., 2024-01-15T17:00:00Z)"
      );
    }

    const response = await this.request(
      {
        service: "Schedules",
        operation: "UpdateEntry",
        resourceType: "schedule_entry",
        isMutation: true,
        projectId,
        resourceId: entryId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/schedule_entries/{entryId}", {
          params: { path: { projectId, entryId } },
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

    return response.entry!;
  }

  /**
   * Gets a specific occurrence of a recurring schedule entry.
   *
   * @param projectId - The project (bucket) ID
   * @param entryId - The schedule entry ID
   * @param date - The occurrence date in YYYY-MM-DD format
   * @returns The schedule entry occurrence
   *
   * @example
   * ```ts
   * const occurrence = await client.schedules.getEntryOccurrence(
   *   projectId,
   *   entryId,
   *   "2024-12-15"
   * );
   * ```
   */
  async getEntryOccurrence(
    projectId: number,
    entryId: number,
    date: string
  ): Promise<ScheduleEntry> {
    if (!date) {
      throw Errors.validation("Occurrence date is required");
    }
    if (!isValidDateFormat(date)) {
      throw Errors.validation("Occurrence date must be in YYYY-MM-DD format");
    }

    const response = await this.request(
      {
        service: "Schedules",
        operation: "GetEntryOccurrence",
        resourceType: "schedule_entry",
        isMutation: false,
        projectId,
        resourceId: entryId,
      },
      () =>
        this.client.GET(
          "/buckets/{projectId}/schedule_entries/{entryId}/occurrences/{date}",
          {
            params: { path: { projectId, entryId, date } },
          }
        )
    );

    return response.entry!;
  }

  /**
   * Updates the settings for a schedule.
   *
   * @param projectId - The project (bucket) ID
   * @param scheduleId - The schedule ID
   * @param req - Settings update parameters
   * @returns The updated schedule
   *
   * @example
   * ```ts
   * // Show todo due dates on the schedule
   * const updated = await client.schedules.updateSettings(projectId, scheduleId, {
   *   includeDueAssignments: true,
   * });
   * ```
   */
  async updateSettings(
    projectId: number,
    scheduleId: number,
    req: UpdateScheduleSettingsRequest
  ): Promise<Schedule> {
    const response = await this.request(
      {
        service: "Schedules",
        operation: "UpdateSettings",
        resourceType: "schedule",
        isMutation: true,
        projectId,
        resourceId: scheduleId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/schedules/{scheduleId}", {
          params: { path: { projectId, scheduleId } },
          body: {
            include_due_assignments: req.includeDueAssignments,
          },
        })
    );

    return response.schedule!;
  }

  /**
   * Moves a schedule entry to the trash.
   * Trashed entries can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param entryId - The schedule entry ID
   *
   * @example
   * ```ts
   * await client.schedules.trashEntry(projectId, entryId);
   * ```
   */
  async trashEntry(projectId: number, entryId: number): Promise<void> {
    await this.request(
      {
        service: "Schedules",
        operation: "TrashEntry",
        resourceType: "schedule_entry",
        isMutation: true,
        projectId,
        resourceId: entryId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: entryId } },
        })
    );
  }
}

// =============================================================================
// Helpers
// =============================================================================

/**
 * Validates that a string is in YYYY-MM-DD format.
 */
function isValidDateFormat(date: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(date);
}

/**
 * Validates that a string is in RFC3339 format.
 */
function isValidRFC3339(date: string): boolean {
  // Basic RFC3339 validation
  // Full format: YYYY-MM-DDTHH:mm:ssZ or YYYY-MM-DDTHH:mm:ss+HH:mm
  const parsed = Date.parse(date);
  if (isNaN(parsed)) return false;

  // Must have T separator and timezone
  return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(date);
}
