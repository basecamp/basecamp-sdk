/**
 * Reports service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Options for upcoming.
 */
export interface UpcomingReportOptions {
  /** Window starts on */
  windowStartsOn?: string;
  /** Window ends on */
  windowEndsOn?: string;
}

/**
 * Options for assigned.
 */
export interface AssignedReportOptions {
  /** Group by "bucket" or "date" */
  groupBy?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Reports operations.
 */
export class ReportsService extends BaseService {

  /**
   * Get account-wide activity feed (progress report)
   * @returns Array of results
   *
   * @example
   * ```ts
   * const result = await client.reports.progress();
   * ```
   */
  async progress(): Promise<components["schemas"]["GetProgressReportResponseContent"]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "GetProgressReport",
        resourceType: "progress_report",
        isMutation: false,
      },
      () =>
        this.client.GET("/reports/progress.json", {
        })
    );
    return response ?? [];
  }

  /**
   * Get upcoming schedule entries within a date window
   * @param options - Optional query parameters
   * @returns The upcoming_schedule
   *
   * @example
   * ```ts
   * const result = await client.reports.upcoming();
   * ```
   */
  async upcoming(options?: UpcomingReportOptions): Promise<components["schemas"]["GetUpcomingScheduleResponseContent"]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "GetUpcomingSchedule",
        resourceType: "upcoming_schedule",
        isMutation: false,
      },
      () =>
        this.client.GET("/reports/schedules/upcoming.json", {
          params: {
            query: { "window_starts_on": options?.windowStartsOn, "window_ends_on": options?.windowEndsOn },
          },
        })
    );
    return response;
  }

  /**
   * Get todos assigned to a specific person
   * @param personId - The person ID
   * @param options - Optional query parameters
   * @returns The assigned_todo
   *
   * @example
   * ```ts
   * const result = await client.reports.assigned(123);
   * ```
   */
  async assigned(personId: number, options?: AssignedReportOptions): Promise<components["schemas"]["GetAssignedTodosResponseContent"]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "GetAssignedTodos",
        resourceType: "assigned_todo",
        isMutation: false,
        resourceId: personId,
      },
      () =>
        this.client.GET("/reports/todos/assigned/{personId}", {
          params: {
            path: { personId },
            query: { "group_by": options?.groupBy },
          },
        })
    );
    return response;
  }

  /**
   * Get overdue todos grouped by lateness
   * @returns The overdue_todo
   *
   * @example
   * ```ts
   * const result = await client.reports.overdue();
   * ```
   */
  async overdue(): Promise<components["schemas"]["GetOverdueTodosResponseContent"]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "GetOverdueTodos",
        resourceType: "overdue_todo",
        isMutation: false,
      },
      () =>
        this.client.GET("/reports/todos/overdue.json", {
        })
    );
    return response;
  }

  /**
   * Get a person's activity timeline
   * @param personId - The person ID
   * @returns The person_progress
   *
   * @example
   * ```ts
   * const result = await client.reports.personProgress(123);
   * ```
   */
  async personProgress(personId: number): Promise<components["schemas"]["GetPersonProgressResponseContent"]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "GetPersonProgress",
        resourceType: "person_progress",
        isMutation: false,
        resourceId: personId,
      },
      () =>
        this.client.GET("/reports/users/progress/{personId}", {
          params: {
            path: { personId },
          },
        })
    );
    return response;
  }
}