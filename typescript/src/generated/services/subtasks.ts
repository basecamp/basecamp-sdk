/**
 * Subtasks service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** CardStep entity from the Basecamp API. */
export type CardStep = components["schemas"]["CardStep"];

/**
 * Options for list.
 */
export interface ListSubtaskOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Request parameters for create.
 */
export interface CreateSubtaskRequest {
  /** Title */
  title: string;
  /** Due date (YYYY-MM-DD) */
  dueOn?: string;
  /** Person IDs to assign to */
  assigneeIds?: number[];
}

/**
 * Request parameters for update.
 */
export interface UpdateSubtaskRequest {
  /** Title */
  title?: string;
  /** Due date (YYYY-MM-DD) */
  dueOn?: string;
  /** Person IDs to assign to */
  assigneeIds?: number[];
}

/**
 * Request parameters for reposition.
 */
export interface RepositionSubtaskRequest {
  /** The 1-based position to move it to */
  position: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Subtasks operations.
 */
export class SubtasksService extends BaseService {

  /**
   * List a recording's subtasks, in position order
   * @param recordingId - The recording ID
   * @param options - Optional query parameters
   * @returns All CardStep across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.subtasks.list(123);
   *
   * // With options
   * const filtered = await client.subtasks.list(123, { page: 1 });
   * ```
   */
  async list(recordingId: number, options?: ListSubtaskOptions): Promise<ListResult<CardStep>> {
    return this.requestPaginated(
      {
        service: "Subtasks",
        operation: "ListSubtasks",
        resourceType: "subtask",
        isMutation: false,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/recordings/{recordingId}/subtasks.json", {
          params: {
            path: { recordingId },
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Create a subtask under a to-do or a card
   * @param recordingId - The recording ID
   * @param req - Subtask creation parameters
   * @returns The CardStep
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.subtasks.create(123, { title: "example" });
   * ```
   */
  async create(recordingId: number, req: CreateSubtaskRequest): Promise<CardStep> {
    if (!req.title) {
      throw Errors.validation("Title is required");
    }
    if (req.dueOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.dueOn)) {
      throw Errors.validation("Due on must be in YYYY-MM-DD format");
    }
    const response = await this.request(
      {
        service: "Subtasks",
        operation: "CreateSubtask",
        resourceType: "subtask",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/subtasks.json", {
          params: {
            path: { recordingId },
          },
          body: {
            title: req.title,
            due_on: req.dueOn,
            assignee_ids: req.assigneeIds,
          },
        })
    );
    return response;
  }

  /**
   * Get a subtask by ID
   * @param subtaskId - The subtask ID
   * @returns The CardStep
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.subtasks.get(123);
   * ```
   */
  async get(subtaskId: number): Promise<CardStep> {
    const response = await this.request(
      {
        service: "Subtasks",
        operation: "GetSubtask",
        resourceType: "subtask",
        isMutation: false,
        resourceId: subtaskId,
      },
      () =>
        this.client.GET("/subtasks/{subtaskId}", {
          params: {
            path: { subtaskId },
          },
        })
    );
    return response;
  }

  /**
   * Update a subtask
   * @param subtaskId - The subtask ID
   * @param req - Subtask update parameters
   * @returns The CardStep
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.subtasks.update(123, { });
   * ```
   */
  async update(subtaskId: number, req: UpdateSubtaskRequest): Promise<CardStep> {
    if (req.dueOn && !/^\d{4}-\d{2}-\d{2}$/.test(req.dueOn)) {
      throw Errors.validation("Due on must be in YYYY-MM-DD format");
    }
    const response = await this.request(
      {
        service: "Subtasks",
        operation: "UpdateSubtask",
        resourceType: "subtask",
        isMutation: true,
        resourceId: subtaskId,
      },
      () =>
        this.client.PUT("/subtasks/{subtaskId}", {
          params: {
            path: { subtaskId },
          },
          body: {
            title: req.title,
            due_on: req.dueOn,
            assignee_ids: req.assigneeIds,
          },
        })
    );
    return response;
  }

  /**
   * Delete a subtask
   * @param subtaskId - The subtask ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.subtasks.delete(123);
   * ```
   */
  async delete(subtaskId: number): Promise<void> {
    await this.request(
      {
        service: "Subtasks",
        operation: "DeleteSubtask",
        resourceType: "subtask",
        isMutation: true,
        resourceId: subtaskId,
      },
      () =>
        this.client.DELETE("/subtasks/{subtaskId}", {
          params: {
            path: { subtaskId },
          },
        })
    );
  }

  /**
   * Mark a subtask as completed
   * @param subtaskId - The subtask ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.subtasks.complete(123);
   * ```
   */
  async complete(subtaskId: number): Promise<void> {
    await this.request(
      {
        service: "Subtasks",
        operation: "CompleteSubtask",
        resourceType: "subtask",
        isMutation: true,
        resourceId: subtaskId,
      },
      () =>
        this.client.POST("/subtasks/{subtaskId}/completion.json", {
          params: {
            path: { subtaskId },
          },
        })
    );
  }

  /**
   * Mark a subtask as not completed
   * @param subtaskId - The subtask ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.subtasks.uncomplete(123);
   * ```
   */
  async uncomplete(subtaskId: number): Promise<void> {
    await this.request(
      {
        service: "Subtasks",
        operation: "UncompleteSubtask",
        resourceType: "subtask",
        isMutation: true,
        resourceId: subtaskId,
      },
      () =>
        this.client.DELETE("/subtasks/{subtaskId}/completion.json", {
          params: {
            path: { subtaskId },
          },
        })
    );
  }

  /**
   * Move a subtask to a new position among its siblings
   * @param subtaskId - The subtask ID
   * @param req - Subtask request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.subtasks.reposition(123, { position: 1 });
   * ```
   */
  async reposition(subtaskId: number, req: RepositionSubtaskRequest): Promise<void> {
    await this.request(
      {
        service: "Subtasks",
        operation: "RepositionSubtask",
        resourceType: "subtask",
        isMutation: true,
        resourceId: subtaskId,
      },
      () =>
        this.client.PUT("/subtasks/{subtaskId}/position.json", {
          params: {
            path: { subtaskId },
          },
          body: {
            position: req.position,
          },
        })
    );
  }
}