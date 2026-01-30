/**
 * Service for Timesheets operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Timesheets operations
 */
export class TimesheetsService extends BaseService {

  /**
   * Get timesheet for a specific recording
   */
  async forRecording(projectId: number, recordingId: number, options?: { from?: string; to?: string; personId?: number }): Promise<components["schemas"]["GetRecordingTimesheetResponseContent"]> {
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
    return response;
  }

  /**
   * Get timesheet for a specific project
   */
  async forProject(projectId: number, options?: { from?: string; to?: string; personId?: number }): Promise<components["schemas"]["GetProjectTimesheetResponseContent"]> {
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
    return response;
  }

  /**
   * Get account-wide timesheet report
   */
  async report(options?: { from?: string; to?: string; personId?: number }): Promise<components["schemas"]["GetTimesheetReportResponseContent"]> {
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
    return response;
  }
}