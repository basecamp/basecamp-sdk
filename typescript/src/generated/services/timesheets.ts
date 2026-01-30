/**
 * Timesheets service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Options for forRecording.
 */
export interface ForRecordingTimesheetOptions {
  /** from */
  from?: string;
  /** to */
  to?: string;
  /** person id */
  personId?: number;
}

/**
 * Options for forProject.
 */
export interface ForProjectTimesheetOptions {
  /** from */
  from?: string;
  /** to */
  to?: string;
  /** person id */
  personId?: number;
}

/**
 * Options for report.
 */
export interface ReportTimesheetOptions {
  /** from */
  from?: string;
  /** to */
  to?: string;
  /** person id */
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
   * @param options - Optional parameters
   * @returns Array of results
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
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/timesheet.json", {
          params: {
            path: { projectId, recordingId },
            query: { from: options?.from, to: options?.to, "person_id": options?.personId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Get timesheet for a specific project
   * @param projectId - The project ID
   * @param options - Optional parameters
   * @returns Array of results
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
        this.client.GET("/buckets/{projectId}/timesheet.json", {
          params: {
            path: { projectId },
            query: { from: options?.from, to: options?.to, "person_id": options?.personId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Get account-wide timesheet report
   * @param options - Optional parameters
   * @returns Array of results
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