/**
 * TodolistGroups service for the Basecamp API.
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

/** TodolistGroup entity from the Basecamp API. */
export type TodolistGroup = components["schemas"]["TodolistGroup"];

/**
 * Request parameters for reposition.
 */
export interface RepositionTodolistGroupRequest {
  /** Position for ordering (1-based) */
  position: number;
}

/**
 * Options for list.
 */
export interface ListTodolistGroupOptions extends PaginationOptions {
}

/**
 * Request parameters for create.
 */
export interface CreateTodolistGroupRequest {
  /** Display name */
  name: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for TodolistGroups operations.
 */
export class TodolistGroupsService extends BaseService {

  /**
   * Reposition a todolist group
   * @param groupId - The group ID
   * @param req - Todolist_group request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.todolistGroups.reposition(123, { position: 1 });
   * ```
   */
  async reposition(groupId: number, req: RepositionTodolistGroupRequest): Promise<void> {
    await this.request(
      {
        service: "TodolistGroups",
        operation: "RepositionTodolistGroup",
        resourceType: "todolist_group",
        isMutation: true,
        resourceId: groupId,
      },
      () =>
        this.client.PUT("/todolists/{groupId}/position.json", {
          params: {
            path: { groupId },
          },
          body: {
            position: req.position,
          },
        })
    );
  }

  /**
   * List groups in a todolist
   * @param todolistId - The todolist ID
   * @param options - Optional query parameters
   * @returns All TodolistGroup across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.todolistGroups.list(123);
   * ```
   */
  async list(todolistId: number, options?: ListTodolistGroupOptions): Promise<ListResult<TodolistGroup>> {
    return this.requestPaginated(
      {
        service: "TodolistGroups",
        operation: "ListTodolistGroups",
        resourceType: "todolist_group",
        isMutation: false,
        resourceId: todolistId,
      },
      () =>
        this.client.GET("/todolists/{todolistId}/groups.json", {
          params: {
            path: { todolistId },
          },
        })
      , options
    );
  }

  /**
   * Create a new group in a todolist
   * @param todolistId - The todolist ID
   * @param req - Todolist_group creation parameters
   * @returns The TodolistGroup
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.todolistGroups.create(123, { name: "My example" });
   * ```
   */
  async create(todolistId: number, req: CreateTodolistGroupRequest): Promise<TodolistGroup> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "TodolistGroups",
        operation: "CreateTodolistGroup",
        resourceType: "todolist_group",
        isMutation: true,
        resourceId: todolistId,
      },
      () =>
        this.client.POST("/todolists/{todolistId}/groups.json", {
          params: {
            path: { todolistId },
          },
          body: {
            name: req.name,
          },
        })
    );
    return response;
  }
}