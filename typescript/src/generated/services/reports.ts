/**
 * Reports service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";

// =============================================================================
// Types
// =============================================================================

/** TimelineEvent entity from the Basecamp API. */
export type TimelineEvent = components["schemas"]["TimelineEvent"];
/** Person entity from the Basecamp API. */
export type Person = components["schemas"]["Person"];

/**
 * Options for progress.
 */
export interface ProgressReportOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Options for assigned.
 */
export interface AssignedReportOptions {
  /** Group by "bucket" or "date" */
  groupBy?: string;
}

/**
 * Options for personProgress.
 */
export interface PersonProgressReportOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
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
   * @param options - Optional query parameters
   * @returns All TimelineEvent across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.reports.progress();
   * ```
   */
  async progress(options?: ProgressReportOptions): Promise<ListResult<TimelineEvent>> {
    return this.requestPaginated(
      {
        service: "Reports",
        operation: "GetProgressReport",
        resourceType: "progress_report",
        isMutation: false,
      },
      () =>
        this.client.GET("/reports/progress.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get upcoming schedule entries and assignable items within a date window.
   * @param windowStartsOn - Inclusive first day of the window, `YYYY-MM-DD`. Required — BC3 answers 400 without it.
   * @param windowEndsOn - Inclusive last day of the window, `YYYY-MM-DD`. Required — BC3 answers 400 without it.
   * @returns The upcoming_schedule
   *
   * @example
   * ```ts
   * const result = await client.reports.upcoming("window_starts_on", "window_ends_on");
   * ```
   */
  async upcoming(windowStartsOn: string, windowEndsOn: string): Promise<components["schemas"]["GetUpcomingScheduleResponseContent"]> {
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
            query: { "window_starts_on": windowStartsOn, "window_ends_on": windowEndsOn },
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
   * @param options - Optional query parameters
   * @returns Wrapper with events as ListResult<TimelineEvent> across all pages
   *
   * @example
   * ```ts
   * const result = await client.reports.personProgress(123);
   * ```
   */
  async personProgress(personId: number, options?: PersonProgressReportOptions): Promise<{ person: Person; events: ListResult<TimelineEvent> }> {
    return this.requestPaginatedWrapped<"events", TimelineEvent>(
      {
        service: "Reports",
        operation: "GetPersonProgress",
        resourceType: "person_progress",
        isMutation: false,
        resourceId: personId,
      },
      () =>
        this.client.GET("/reports/users/progress/{personId}.json", {
          params: {
            path: { personId },
            query: { page: options?.page },
          },
        })
      , "events", options
    ) as unknown as { person: Person; events: ListResult<TimelineEvent> };
  }
}