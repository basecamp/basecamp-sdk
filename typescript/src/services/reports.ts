/**
 * Reports service for the Basecamp API.
 *
 * Provides access to various report types including timesheet reports.
 *
 * @example
 * ```ts
 * const entries = await client.reports.timesheet();
 * const projectEntries = await client.reports.projectTimesheet(projectId);
 * ```
 */

import { BaseService } from "./base.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A timesheet entry representing logged time.
 */
export type TimesheetEntry = components["schemas"]["TimesheetEntry"];

/**
 * Options for timesheet reports.
 */
export interface TimesheetReportOptions {
  /** Filter entries on or after this date (ISO 8601 format, e.g., "2024-01-01") */
  from?: string;
  /** Filter entries on or before this date (ISO 8601 format, e.g., "2024-01-31") */
  to?: string;
  /** Filter entries by a specific person ID */
  personId?: number;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for accessing Basecamp reports.
 */
export class ReportsService extends BaseService {
  /**
   * Returns the account-wide timesheet report.
   * This includes time entries across all projects in the account.
   *
   * @param options - Optional filters for the report
   * @returns Array of timesheet entries
   *
   * @example
   * ```ts
   * // Get all timesheet entries
   * const entries = await client.reports.timesheet();
   *
   * // Get entries for a specific date range
   * const entries = await client.reports.timesheet({
   *   from: "2024-01-01",
   *   to: "2024-01-31",
   * });
   *
   * // Get entries for a specific person
   * const entries = await client.reports.timesheet({
   *   personId: 12345,
   * });
   * ```
   */
  async timesheet(options?: TimesheetReportOptions): Promise<TimesheetEntry[]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "Timesheet",
        resourceType: "timesheet_entry",
        isMutation: false,
      },
      () =>
        this.client.GET("/reports/timesheet.json", {
          params: {
            query: {
              from: options?.from,
              to: options?.to,
              person_id: options?.personId,
            },
          },
        })
    );

    return response ?? [];
  }

  /**
   * Returns the timesheet report for a specific project.
   *
   * @param projectId - The project (bucket) ID
   * @param options - Optional filters for the report
   * @returns Array of timesheet entries
   *
   * @example
   * ```ts
   * // Get all timesheet entries for a project
   * const entries = await client.reports.projectTimesheet(projectId);
   *
   * // Get entries for a specific date range
   * const entries = await client.reports.projectTimesheet(projectId, {
   *   from: "2024-01-01",
   *   to: "2024-01-31",
   * });
   * ```
   */
  async projectTimesheet(
    projectId: number,
    options?: TimesheetReportOptions
  ): Promise<TimesheetEntry[]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "ProjectTimesheet",
        resourceType: "timesheet_entry",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/timesheet.json", {
          params: {
            path: { projectId },
            query: {
              from: options?.from,
              to: options?.to,
              person_id: options?.personId,
            },
          },
        })
    );

    return response ?? [];
  }

  /**
   * Returns the timesheet report for a specific recording within a project.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID (e.g., a todo or message)
   * @param options - Optional filters for the report
   * @returns Array of timesheet entries
   *
   * @example
   * ```ts
   * // Get timesheet entries for a specific todo
   * const entries = await client.reports.recordingTimesheet(projectId, todoId);
   * ```
   */
  async recordingTimesheet(
    projectId: number,
    recordingId: number,
    options?: TimesheetReportOptions
  ): Promise<TimesheetEntry[]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "RecordingTimesheet",
        resourceType: "timesheet_entry",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/timesheet.json", {
          params: {
            path: { projectId, recordingId },
            query: {
              from: options?.from,
              to: options?.to,
              person_id: options?.personId,
            },
          },
        })
    );

    return response ?? [];
  }
}
