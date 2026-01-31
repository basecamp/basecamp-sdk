/**
 * Todolists service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Todolist entity from the Basecamp API. */
export type Todolist = components["schemas"]["Todolist"];

/**
 * Request parameters for update.
 */
export interface UpdateTodolistRequest {
  /** Name (required for both Todolist and TodolistGroup) */
  name?: string;
  /** Description (Todolist only, ignored for groups) */
  description?: string;
}

/**
 * Options for list.
 */
export interface ListTodolistOptions {
  /** active|archived|trashed */
  status?: string;
}

/**
 * Request parameters for create.
 */
export interface CreateTodolistRequest {
  /** name */
  name: string;
  /** description */
  description?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Todolists operations.
 */
export class TodolistsService extends BaseService {

  /**
   * Get a single todolist or todolist group by id
   * @param projectId - The project ID
   * @param id - The id
   * @returns The todolist_or_group
   */
  async get(projectId: number, id: number): Promise<components["schemas"]["GetTodolistOrGroupResponseContent"]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "GetTodolistOrGroup",
        resourceType: "todolist_or_group",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todolists/{id}", {
          params: {
            path: { projectId, id },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing todolist or todolist group
   * @param projectId - The project ID
   * @param id - The id
   * @param req - Request parameters
   * @returns The todolist_or_group
   */
  async update(projectId: number, id: number, req: UpdateTodolistRequest): Promise<components["schemas"]["UpdateTodolistOrGroupResponseContent"]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "UpdateTodolistOrGroup",
        resourceType: "todolist_or_group",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/todolists/{id}", {
          params: {
            path: { projectId, id },
          },
          body: req as any,
        })
    );
    return response;
  }

  /**
   * List todolists in a todoset
   * @param projectId - The project ID
   * @param todosetId - The todoset ID
   * @param options - Optional parameters
   * @returns Array of Todolist
   */
  async list(projectId: number, todosetId: number, options?: ListTodolistOptions): Promise<Todolist[]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "ListTodolists",
        resourceType: "todolist",
        isMutation: false,
        projectId,
        resourceId: todosetId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todosets/{todosetId}/todolists.json", {
          params: {
            path: { projectId, todosetId },
            query: { status: options?.status },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new todolist in a todoset
   * @param projectId - The project ID
   * @param todosetId - The todoset ID
   * @param req - Request parameters
   * @returns The Todolist
   *
   * @example
   * ```ts
   * const result = await client.todolists.create(123, 123, { ... });
   * ```
   */
  async create(projectId: number, todosetId: number, req: CreateTodolistRequest): Promise<Todolist> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "CreateTodolist",
        resourceType: "todolist",
        isMutation: true,
        projectId,
        resourceId: todosetId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/todosets/{todosetId}/todolists.json", {
          params: {
            path: { projectId, todosetId },
          },
          body: req as any,
        })
    );
    return response;
  }
}