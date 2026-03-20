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

/** MyAssignment entity from the Basecamp API. */
export type MyAssignment = components["schemas"]["MyAssignment"];
/** TimelineEvent entity from the Basecamp API. */
export type TimelineEvent = components["schemas"]["TimelineEvent"];
/** Person entity from the Basecamp API. */
export type Person = components["schemas"]["Person"];

/**
 * Options for myAssignmentsDue.
 */
export interface MyAssignmentsDueReportOptions {
  /** Filter by due scope: overdue, due_today, due_tomorrow, due_later_this_week, due_next_week, due_later */
  scope?: string;
}

/**
 * Options for progress.
 */
export interface ProgressReportOptions extends PaginationOptions {
}

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

/**
 * Options for personProgress.
 */
export interface PersonProgressReportOptions extends PaginationOptions {
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Reports operations.
 */
export class ReportsService extends BaseService {

  /**
   * Get the current user's assignments grouped into priorities and non-priorities
   * @returns The my_assignment
   *
   * @example
   * ```ts
   * const result = await client.reports.myAssignments();
   * ```
   */
  async myAssignments(): Promise<components["schemas"]["GetMyAssignmentsResponseContent"]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "GetMyAssignments",
        resourceType: "my_assignment",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/assignments.json", {
        })
    );
    return response;
  }

  /**
   * Get the current user's completed assignments
   * @returns Array of MyAssignment
   *
   * @example
   * ```ts
   * const result = await client.reports.myAssignmentsCompleted();
   * ```
   */
  async myAssignmentsCompleted(): Promise<MyAssignment[]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "GetMyAssignmentsCompleted",
        resourceType: "my_assignments_completed",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/assignments/completed.json", {
        })
    );
    return response ?? [];
  }

  /**
   * Get the current user's due assignments filtered by scope
   * @param options - Optional query parameters
   * @returns Array of MyAssignment
   *
   * @example
   * ```ts
   * const result = await client.reports.myAssignmentsDue();
   * ```
   */
  async myAssignmentsDue(options?: MyAssignmentsDueReportOptions): Promise<MyAssignment[]> {
    const response = await this.request(
      {
        service: "Reports",
        operation: "GetMyAssignmentsDue",
        resourceType: "my_assignments_due",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/assignments/due.json", {
          params: {
            query: { scope: options?.scope },
          },
        })
    );
    return response ?? [];
  }

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
        })
      , options
    );
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
          },
        })
      , "events", options
    ) as unknown as { person: Person; events: ListResult<TimelineEvent> };
  }
}