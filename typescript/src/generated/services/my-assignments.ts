/**
 * MyAssignments service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Options for myDueAssignments.
 */
export interface MyDueAssignmentsMyAssignmentOptions {
  /** Filter by due date range: overdue, due_today, due_tomorrow,
due_later_this_week, due_next_week, due_later */
  scope?: string;
}

/**
 * Request parameters for prioritizeAssignment.
 */
export interface PrioritizeAssignmentMyAssignmentRequest {
  /** The recording id to prioritize. */
  id: number;
}

/**
 * Request parameters for reorderUpNext.
 */
export interface ReorderUpNextMyAssignmentRequest {
  /** The recording id to move, chosen the same way as when prioritizing. */
  sourceId: number;
  /** The 1-based position to move it to. */
  position: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for MyAssignments operations.
 */
export class MyAssignmentsService extends BaseService {

  /**
   * Get the current user's active assignments grouped into priorities and non_priorities.
   * @returns The my_assignment
   *
   * @example
   * ```ts
   * const result = await client.myAssignments.myAssignments();
   * ```
   */
  async myAssignments(): Promise<components["schemas"]["GetMyAssignmentsResponseContent"]> {
    const response = await this.request(
      {
        service: "MyAssignments",
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
   * Get the current user's completed assignments.
   * @returns Array of results
   *
   * @example
   * ```ts
   * const result = await client.myAssignments.myCompletedAssignments();
   * ```
   */
  async myCompletedAssignments(): Promise<components["schemas"]["GetMyCompletedAssignmentsResponseContent"]> {
    const response = await this.request(
      {
        service: "MyAssignments",
        operation: "GetMyCompletedAssignments",
        resourceType: "my_completed_assignment",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/assignments/completed.json", {
        })
    );
    return response ?? [];
  }

  /**
   * Get the current user's assignments filtered by due date scope.
   * @param options - Optional query parameters
   * @returns Array of results
   *
   * @example
   * ```ts
   * const result = await client.myAssignments.myDueAssignments();
   * ```
   */
  async myDueAssignments(options?: MyDueAssignmentsMyAssignmentOptions): Promise<components["schemas"]["GetMyDueAssignmentsResponseContent"]> {
    const response = await this.request(
      {
        service: "MyAssignments",
        operation: "GetMyDueAssignments",
        resourceType: "my_due_assignment",
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
   * Add a recording to Up Next — the current user's ordered list of prioritized
   * @param req - Resource request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.myAssignments.prioritizeAssignment({ id: 1 });
   * ```
   */
  async prioritizeAssignment(req: PrioritizeAssignmentMyAssignmentRequest): Promise<void> {
    await this.request(
      {
        service: "MyAssignments",
        operation: "PrioritizeAssignment",
        resourceType: "resource",
        isMutation: true,
      },
      () =>
        this.client.POST("/my/priorities.json", {
          body: {
            id: req.id,
          },
        })
    );
  }

  /**
   * Remove a recording from Up Next. Exact-target:
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.myAssignments.deprioritizeAssignment(123);
   * ```
   */
  async deprioritizeAssignment(recordingId: number): Promise<void> {
    await this.request(
      {
        service: "MyAssignments",
        operation: "DeprioritizeAssignment",
        resourceType: "resource",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.DELETE("/my/priorities/{recordingId}", {
          params: {
            path: { recordingId },
          },
        })
    );
  }

  /**
   * Move an already-prioritized recording to a new 1-based position in Up Next
   * @param req - Resource request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.myAssignments.reorderUpNext({ sourceId: 1, position: 1 });
   * ```
   */
  async reorderUpNext(req: ReorderUpNextMyAssignmentRequest): Promise<void> {
    await this.request(
      {
        service: "MyAssignments",
        operation: "ReorderUpNext",
        resourceType: "resource",
        isMutation: true,
      },
      () =>
        this.client.POST("/my/priority_moves.json", {
          body: {
            source_id: req.sourceId,
            position: req.position,
          },
        })
    );
  }
}