/**
 * TodolistGroups service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** TodolistGroup entity from the Basecamp API. */
export type TodolistGroup = components["schemas"]["TodolistGroup"];

/**
 * Request parameters for reposition.
 */
export interface RepositionTodolistGroupRequest {
  /** position */
  position: number;
}

/**
 * Request parameters for create.
 */
export interface CreateTodolistGroupRequest {
  /** name */
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
   * @param projectId - The project ID
   * @param groupId - The group ID
   * @param req - Request parameters
   * @returns void
   */
  async reposition(projectId: number, groupId: number, req: RepositionTodolistGroupRequest): Promise<void> {
    await this.request(
      {
        service: "TodolistGroups",
        operation: "RepositionTodolistGroup",
        resourceType: "todolist_group",
        isMutation: true,
        projectId,
        resourceId: groupId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todolists/{groupId}/position.json", {
          params: {
            path: { projectId, groupId },
          },
          body: req as any,
        })
    );
  }

  /**
   * List groups in a todolist
   * @param projectId - The project ID
   * @param todolistId - The todolist ID
   * @returns Array of TodolistGroup
   */
  async list(projectId: number, todolistId: number): Promise<TodolistGroup[]> {
    const response = await this.request(
      {
        service: "TodolistGroups",
        operation: "ListTodolistGroups",
        resourceType: "todolist_group",
        isMutation: false,
        projectId,
        resourceId: todolistId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todolists/{todolistId}/groups.json", {
          params: {
            path: { projectId, todolistId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new group in a todolist
   * @param projectId - The project ID
   * @param todolistId - The todolist ID
   * @param req - Request parameters
   * @returns The TodolistGroup
   *
   * @example
   * ```ts
   * const result = await client.todolistGroups.create(123, 123, { ... });
   * ```
   */
  async create(projectId: number, todolistId: number, req: CreateTodolistGroupRequest): Promise<TodolistGroup> {
    const response = await this.request(
      {
        service: "TodolistGroups",
        operation: "CreateTodolistGroup",
        resourceType: "todolist_group",
        isMutation: true,
        projectId,
        resourceId: todolistId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todolists/{todolistId}/groups.json", {
          params: {
            path: { projectId, todolistId },
          },
          body: req as any,
        })
    );
    return response;
  }
}