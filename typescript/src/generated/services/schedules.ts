/**
 * Service for Schedules operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Schedules operations
 */
export class SchedulesService extends BaseService {

  /**
   * Get a single schedule entry by id
   */
  async getEntry(projectId: number, entryId: number): Promise<components["schemas"]["GetScheduleEntryResponseContent"]> {
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
   */
  async updateEntry(projectId: number, entryId: number, req: components["schemas"]["UpdateScheduleEntryRequestContent"]): Promise<components["schemas"]["UpdateScheduleEntryResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a specific occurrence of a recurring schedule entry
   */
  async getEntryOccurrence(projectId: number, entryId: number, date: string): Promise<components["schemas"]["GetScheduleEntryOccurrenceResponseContent"]> {
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
   */
  async get(projectId: number, scheduleId: number): Promise<components["schemas"]["GetScheduleResponseContent"]> {
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
   */
  async updateSettings(projectId: number, scheduleId: number, req: components["schemas"]["UpdateScheduleSettingsRequestContent"]): Promise<components["schemas"]["UpdateScheduleSettingsResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * List entries on a schedule
   */
  async listEntries(projectId: number, scheduleId: number, options?: { status?: string }): Promise<components["schemas"]["ListScheduleEntriesResponseContent"]> {
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
   */
  async createEntry(projectId: number, scheduleId: number, req: components["schemas"]["CreateScheduleEntryRequestContent"]): Promise<components["schemas"]["CreateScheduleEntryResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }
}