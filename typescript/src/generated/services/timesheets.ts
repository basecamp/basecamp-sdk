/**
 * Timesheets service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Options for forRecording.
 */
export interface ForRecordingTimesheetOptions {
  /** From */
  from?: string;
  /** To */
  to?: string;
  /** Person id */
  personId?: number;
}

/**
 * Request parameters for create.
 */
export interface CreateTimesheetRequest {
  /** Date */
  date: string;
  /** Hours */
  hours: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Person id */
  personId?: number;
}

/**
 * Options for forProject.
 */
export interface ForProjectTimesheetOptions {
  /** From */
  from?: string;
  /** To */
  to?: string;
  /** Person id */
  personId?: number;
}

/**
 * Request parameters for update.
 */
export interface UpdateTimesheetRequest {
  /** Date */
  date?: string;
  /** Hours */
  hours?: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Person id */
  personId?: number;
}

/**
 * Options for report.
 */
export interface ReportTimesheetOptions {
  /** From */
  from?: string;
  /** To */
  to?: string;
  /** Person id */
  personId?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Timesheets operations.
 */
export class TimesheetsService extends BaseService {

  /**
   * Get timesheet for a specific recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param options - Optional query parameters
   * @returns Array of results
   *
   * @example
   * ```ts
   * const result = await client.timesheets.forRecording(123, 123);
   * ```
   */
  async forRecording(projectId: number, recordingId: number, options?: ForRecordingTimesheetOptions): Promise<components["schemas"]["GetRecordingTimesheetResponseContent"]> {
    const response = await this.request(
      {
        service: "Timesheets",
        operation: "GetRecordingTimesheet",
        resourceType: "recording_timesheet",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/projects/{projectId}/recordings/{recordingId}/timesheet.json", {
          params: {
            path: { projectId, recordingId },
            query: { from: options?.from, to: options?.to, "person_id": options?.personId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a timesheet entry on a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param req - Timesheet_entry creation parameters
   * @returns The timesheet_entry
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.timesheets.create(123, 123, { date: "example", hours: "example" });
   * ```
   */
  async create(projectId: number, recordingId: number, req: CreateTimesheetRequest): Promise<components["schemas"]["CreateTimesheetEntryResponseContent"]> {
    if (!req.date) {
      throw Errors.validation("Date is required");
    }
    if (!req.hours) {
      throw Errors.validation("Hours is required");
    }
    const response = await this.request(
      {
        service: "Timesheets",
        operation: "CreateTimesheetEntry",
        resourceType: "timesheet_entry",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/projects/{projectId}/recordings/{recordingId}/timesheet/entries.json", {
          params: {
            path: { projectId, recordingId },
          },
          body: {
            date: req.date,
            hours: req.hours,
            description: req.description,
            person_id: req.personId,
          },
        })
    );
    return response;
  }

  /**
   * Get timesheet for a specific project
   * @param projectId - The project ID
   * @param options - Optional query parameters
   * @returns Array of results
   *
   * @example
   * ```ts
   * const result = await client.timesheets.forProject(123);
   * ```
   */
  async forProject(projectId: number, options?: ForProjectTimesheetOptions): Promise<components["schemas"]["GetProjectTimesheetResponseContent"]> {
    const response = await this.request(
      {
        service: "Timesheets",
        operation: "GetProjectTimesheet",
        resourceType: "project_timesheet",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/projects/{projectId}/timesheet.json", {
          params: {
            path: { projectId },
            query: { from: options?.from, to: options?.to, "person_id": options?.personId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Get a single timesheet entry
   * @param projectId - The project ID
   * @param entryId - The entry ID
   * @returns The timesheet_entry
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.timesheets.get(123, 123);
   * ```
   */
  async get(projectId: number, entryId: number): Promise<components["schemas"]["GetTimesheetEntryResponseContent"]> {
    const response = await this.request(
      {
        service: "Timesheets",
        operation: "GetTimesheetEntry",
        resourceType: "timesheet_entry",
        isMutation: false,
        projectId,
        resourceId: entryId,
      },
      () =>
        this.client.GET("/projects/{projectId}/timesheet/entries/{entryId}", {
          params: {
            path: { projectId, entryId },
          },
        })
    );
    return response;
  }

  /**
   * Update a timesheet entry
   * @param projectId - The project ID
   * @param entryId - The entry ID
   * @param req - Timesheet_entry update parameters
   * @returns The timesheet_entry
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.timesheets.update(123, 123, { });
   * ```
   */
  async update(projectId: number, entryId: number, req: UpdateTimesheetRequest): Promise<components["schemas"]["UpdateTimesheetEntryResponseContent"]> {
    const response = await this.request(
      {
        service: "Timesheets",
        operation: "UpdateTimesheetEntry",
        resourceType: "timesheet_entry",
        isMutation: true,
        projectId,
        resourceId: entryId,
      },
      () =>
        this.client.PUT("/projects/{projectId}/timesheet/entries/{entryId}", {
          params: {
            path: { projectId, entryId },
          },
          body: {
            date: req.date,
            hours: req.hours,
            description: req.description,
            person_id: req.personId,
          },
        })
    );
    return response;
  }

  /**
   * Get account-wide timesheet report
   * @param options - Optional query parameters
   * @returns Array of results
   *
   * @example
   * ```ts
   * const result = await client.timesheets.report();
   * ```
   */
  async report(options?: ReportTimesheetOptions): Promise<components["schemas"]["GetTimesheetReportResponseContent"]> {
    const response = await this.request(
      {
        service: "Timesheets",
        operation: "GetTimesheetReport",
        resourceType: "timesheet_report",
        isMutation: false,
      },
      () =>
        this.client.GET("/reports/timesheet.json", {
          params: {
            query: { from: options?.from, to: options?.to, "person_id": options?.personId },
          },
        })
    );
    return response ?? [];
  }
}