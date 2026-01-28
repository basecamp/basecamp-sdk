/**
 * Todolists service for the Basecamp API.
 *
 * Todolists are containers for todos within a project's todoset.
 * Each project has one todoset which can contain multiple todolists.
 *
 * @example
 * ```ts
 * const lists = await client.todolists.list(projectId, todosetId);
 * const list = await client.todolists.get(projectId, todolistId);
 * await client.todolists.create(projectId, todosetId, { name: "Sprint 1" });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp todolist.
 */
export type Todolist = components["schemas"]["Todolist"];

/**
 * Options for listing todolists.
 */
export interface TodolistListOptions {
  /**
   * Filter by status: "archived" or "trashed".
   * Empty returns active todolists.
   */
  status?: "archived" | "trashed";
}

/**
 * Request to create a new todolist.
 */
export interface CreateTodolistRequest {
  /** Todolist name (required) */
  name: string;
  /** Description in HTML (optional) */
  description?: string;
}

/**
 * Request to update an existing todolist.
 */
export interface UpdateTodolistRequest {
  /** Todolist name (optional) */
  name?: string;
  /** Description in HTML (optional) */
  description?: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp todolists.
 */
export class TodolistsService extends BaseService {
  /**
   * Lists all todolists in a todoset.
   *
   * @param projectId - The project (bucket) ID
   * @param todosetId - The todoset ID
   * @param options - Optional filters
   * @returns Array of todolists
   *
   * @example
   * ```ts
   * // List active todolists
   * const lists = await client.todolists.list(projectId, todosetId);
   *
   * // List archived todolists
   * const archived = await client.todolists.list(projectId, todosetId, { status: "archived" });
   * ```
   */
  async list(
    projectId: number,
    todosetId: number,
    options?: TodolistListOptions
  ): Promise<Todolist[]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "List",
        resourceType: "todolist",
        isMutation: false,
        projectId,
        resourceId: todosetId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todosets/{todosetId}/todolists.json", {
          params: {
            path: { projectId, todosetId },
            query: options?.status ? { status: options.status } : undefined,
          },
        })
    );

    return response?.todolists ?? [];
  }

  /**
   * Gets a todolist by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param todolistId - The todolist ID
   * @returns The todolist
   * @throws BasecampError with code "not_found" if todolist doesn't exist
   *
   * @example
   * ```ts
   * const list = await client.todolists.get(projectId, todolistId);
   * console.log(list.name, list.completed_ratio);
   * ```
   */
  async get(projectId: number, todolistId: number): Promise<Todolist> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "Get",
        resourceType: "todolist",
        isMutation: false,
        projectId,
        resourceId: todolistId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todolists/{id}", {
          params: { path: { projectId, id: todolistId } },
        })
    );

    // The response is a union type (TodolistOrGroup) - extract the todolist
    const result = response.result!;
    if ("todolist" in result && result.todolist) {
      return result.todolist as unknown as Todolist;
    }
    // If it's a group, treat it as a todolist (they share the same structure)
    if ("todolist_group" in result && result.todolist_group) {
      return result.todolist_group as unknown as Todolist;
    }
    // Fallback - return result as todolist
    return result as unknown as Todolist;
  }

  /**
   * Creates a new todolist in a todoset.
   *
   * @param projectId - The project (bucket) ID
   * @param todosetId - The todoset ID
   * @param req - Todolist creation parameters
   * @returns The created todolist
   * @throws BasecampError with code "validation" if name is missing
   *
   * @example
   * ```ts
   * const list = await client.todolists.create(projectId, todosetId, {
   *   name: "Sprint 1",
   *   description: "<em>Tasks for the first sprint</em>",
   * });
   * ```
   */
  async create(
    projectId: number,
    todosetId: number,
    req: CreateTodolistRequest
  ): Promise<Todolist> {
    if (!req.name) {
      throw Errors.validation("Todolist name is required");
    }

    const response = await this.request(
      {
        service: "Todolists",
        operation: "Create",
        resourceType: "todolist",
        isMutation: true,
        projectId,
        resourceId: todosetId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todosets/{todosetId}/todolists.json", {
          params: { path: { projectId, todosetId } },
          body: {
            name: req.name,
            description: req.description,
          },
        })
    );

    return response.todolist!;
  }

  /**
   * Updates an existing todolist.
   *
   * @param projectId - The project (bucket) ID
   * @param todolistId - The todolist ID
   * @param req - Todolist update parameters
   * @returns The updated todolist
   *
   * @example
   * ```ts
   * const list = await client.todolists.update(projectId, todolistId, {
   *   name: "Updated Sprint 1",
   * });
   * ```
   */
  async update(
    projectId: number,
    todolistId: number,
    req: UpdateTodolistRequest
  ): Promise<Todolist> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "Update",
        resourceType: "todolist",
        isMutation: true,
        projectId,
        resourceId: todolistId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todolists/{id}", {
          params: { path: { projectId, id: todolistId } },
          body: {
            name: req.name,
            description: req.description,
          },
        })
    );

    // The response is a union type - extract the todolist
    const result = response.result!;
    if ("todolist" in result && result.todolist) {
      return result.todolist as unknown as Todolist;
    }
    if ("todolist_group" in result && result.todolist_group) {
      return result.todolist_group as unknown as Todolist;
    }
    return result as unknown as Todolist;
  }

  /**
   * Moves a todolist to the trash.
   * Trashed todolists can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param todolistId - The todolist ID
   *
   * @example
   * ```ts
   * await client.todolists.trash(projectId, todolistId);
   * ```
   */
  async trash(projectId: number, todolistId: number): Promise<void> {
    await this.request(
      {
        service: "Todolists",
        operation: "Trash",
        resourceType: "todolist",
        isMutation: true,
        projectId,
        resourceId: todolistId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: todolistId } },
        })
    );
  }
}
