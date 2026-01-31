/**
 * Todolist Groups service for the Basecamp API.
 *
 * Todolist groups are organizational folders within a todolist,
 * allowing you to organize todos into logical sections.
 *
 * @example
 * ```ts
 * const groups = await client.todolistGroups.list(projectId, todolistId);
 * const group = await client.todolistGroups.create(projectId, todolistId, {
 *   name: "Phase 1",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A todolist group (organizational folder within a todolist).
 */
export type TodolistGroup = components["schemas"]["TodolistGroup"];

/**
 * Request to create a new todolist group.
 */
export interface CreateTodolistGroupRequest {
  /** Group name (required) */
  name: string;
}

/**
 * Request to update an existing todolist group.
 */
export interface UpdateTodolistGroupRequest {
  /** Group name */
  name?: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp todolist groups.
 */
export class TodolistGroupsService extends BaseService {
  /**
   * Lists all groups in a todolist.
   *
   * @param projectId - The project (bucket) ID
   * @param todolistId - The todolist ID
   * @returns Array of todolist groups
   *
   * @example
   * ```ts
   * const groups = await client.todolistGroups.list(projectId, todolistId);
   * for (const group of groups) {
   *   console.log(group.name, group.completed_ratio);
   * }
   * ```
   */
  async list(projectId: number, todolistId: number): Promise<TodolistGroup[]> {
    const response = await this.request(
      {
        service: "TodolistGroups",
        operation: "List",
        resourceType: "todolist_group",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todolists/{todolistId}/groups.json", {
          params: { path: { projectId, todolistId } },
        })
    );

    return response ?? [];
  }

  /**
   * Gets a todolist group by ID.
   *
   * Note: Groups share an endpoint with todolists. This method fetches
   * the item and verifies it's a group.
   *
   * @param projectId - The project (bucket) ID
   * @param groupId - The group ID
   * @returns The todolist group
   * @throws BasecampError with code "not_found" if group doesn't exist
   *
   * @example
   * ```ts
   * const group = await client.todolistGroups.get(projectId, groupId);
   * console.log(group.name, group.todos_url);
   * ```
   */
  async get(projectId: number, groupId: number): Promise<TodolistGroup> {
    const response = await this.request(
      {
        service: "TodolistGroups",
        operation: "Get",
        resourceType: "todolist_group",
        isMutation: false,
        projectId,
        resourceId: groupId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todolists/{id}", {
          params: { path: { projectId, id: groupId } },
        })
    );

    // The response is a union type - check if it's a group
    const result = response as { group?: TodolistGroup; todolist?: unknown };
    if (result.group) {
      return result.group;
    }

    // If it's a todolist instead of a group, throw not found
    throw Errors.notFound("Todolist group not found");
  }

  /**
   * Creates a new group in a todolist.
   *
   * @param projectId - The project (bucket) ID
   * @param todolistId - The todolist ID
   * @param req - Group creation parameters
   * @returns The created group
   * @throws BasecampError with code "validation" if name is missing
   *
   * @example
   * ```ts
   * const group = await client.todolistGroups.create(projectId, todolistId, {
   *   name: "Phase 1 Tasks",
   * });
   * ```
   */
  async create(
    projectId: number,
    todolistId: number,
    req: CreateTodolistGroupRequest
  ): Promise<TodolistGroup> {
    if (!req.name) {
      throw Errors.validation("Group name is required");
    }

    const response = await this.request(
      {
        service: "TodolistGroups",
        operation: "Create",
        resourceType: "todolist_group",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todolists/{todolistId}/groups.json", {
          params: { path: { projectId, todolistId } },
          body: {
            name: req.name,
          },
        })
    );

    return response;
  }

  /**
   * Updates an existing todolist group.
   *
   * Note: Groups share an update endpoint with todolists.
   *
   * @param projectId - The project (bucket) ID
   * @param groupId - The group ID
   * @param req - Group update parameters
   * @returns The updated group
   *
   * @example
   * ```ts
   * const group = await client.todolistGroups.update(projectId, groupId, {
   *   name: "Updated Phase Name",
   * });
   * ```
   */
  async update(
    projectId: number,
    groupId: number,
    req: UpdateTodolistGroupRequest
  ): Promise<TodolistGroup> {
    const response = await this.request(
      {
        service: "TodolistGroups",
        operation: "Update",
        resourceType: "todolist_group",
        isMutation: true,
        projectId,
        resourceId: groupId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todolists/{id}", {
          params: { path: { projectId, id: groupId } },
          body: {
            name: req.name,
          },
        })
    );

    // The response is a union type - check if it's a group
    const result = response as { group?: TodolistGroup; todolist?: unknown };
    if (result.group) {
      return result.group;
    }

    throw Errors.notFound("Todolist group not found");
  }

  /**
   * Changes the position of a group within its todolist.
   *
   * @param projectId - The project (bucket) ID
   * @param groupId - The group ID
   * @param position - The new position (1-based, 1 = first position)
   *
   * @example
   * ```ts
   * // Move group to first position
   * await client.todolistGroups.reposition(projectId, groupId, 1);
   * ```
   */
  async reposition(projectId: number, groupId: number, position: number): Promise<void> {
    if (position < 1) {
      throw Errors.validation("Position must be at least 1");
    }

    await this.request(
      {
        service: "TodolistGroups",
        operation: "Reposition",
        resourceType: "todolist_group",
        isMutation: true,
        projectId,
        resourceId: groupId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todolists/{groupId}/position.json", {
          params: { path: { projectId, groupId } },
          body: {
            position,
          },
        })
    );
  }

  /**
   * Moves a todolist group to the trash.
   * Trashed groups can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param groupId - The group ID
   *
   * @example
   * ```ts
   * await client.todolistGroups.trash(projectId, groupId);
   * ```
   */
  async trash(projectId: number, groupId: number): Promise<void> {
    await this.request(
      {
        service: "TodolistGroups",
        operation: "Trash",
        resourceType: "todolist_group",
        isMutation: true,
        projectId,
        resourceId: groupId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: groupId } },
        })
    );
  }
}
