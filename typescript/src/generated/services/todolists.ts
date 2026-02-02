/**
 * Todolists service for the Basecamp API.
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
export interface ListTodolistOptions extends PaginationOptions {
  /** Filter by status */
  status?: "active" | "archived" | "trashed";
}

/**
 * Request parameters for create.
 */
export interface CreateTodolistRequest {
  /** Display name */
  name: string;
  /** Rich text description (HTML) */
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
   * @param id - The id
   * @returns The todolist_or_group
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.todolists.get(123);
   * ```
   */
  async get(id: number): Promise<components["schemas"]["GetTodolistOrGroupResponseContent"]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "GetTodolistOrGroup",
        resourceType: "todolist_or_group",
        isMutation: false,
      },
      () =>
        this.client.GET("/todolists/{id}", {
          params: {
            path: { id },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing todolist or todolist group
   * @param id - The id
   * @param req - Todolist_or_group update parameters
   * @returns The todolist_or_group
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.todolists.update(123, { });
   * ```
   */
  async update(id: number, req: UpdateTodolistRequest): Promise<components["schemas"]["UpdateTodolistOrGroupResponseContent"]> {
    const response = await this.request(
      {
        service: "Todolists",
        operation: "UpdateTodolistOrGroup",
        resourceType: "todolist_or_group",
        isMutation: true,
      },
      () =>
        this.client.PUT("/todolists/{id}", {
          params: {
            path: { id },
          },
          body: {
            name: req.name,
            description: req.description,
          },
        })
    );
    return response;
  }

  /**
   * List todolists in a todoset
   * @param todosetId - The todoset ID
   * @param options - Optional query parameters
   * @returns All Todolist across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.todolists.list(123);
   *
   * // With options
   * const filtered = await client.todolists.list(123, { status: "active" });
   * ```
   */
  async list(todosetId: number, options?: ListTodolistOptions): Promise<ListResult<Todolist>> {
    return this.requestPaginated(
      {
        service: "Todolists",
        operation: "ListTodolists",
        resourceType: "todolist",
        isMutation: false,
        resourceId: todosetId,
      },
      () =>
        this.client.GET("/todosets/{todosetId}/todolists.json", {
          params: {
            path: { todosetId },
            query: { status: options?.status },
          },
        })
      , options
    );
  }

  /**
   * Create a new todolist in a todoset
   * @param todosetId - The todoset ID
   * @param req - Todolist creation parameters
   * @returns The Todolist
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.todolists.create(123, { name: "My example" });
   * ```
   */
  async create(todosetId: number, req: CreateTodolistRequest): Promise<Todolist> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "Todolists",
        operation: "CreateTodolist",
        resourceType: "todolist",
        isMutation: true,
        resourceId: todosetId,
      },
      () =>
        this.client.POST("/todosets/{todosetId}/todolists.json", {
          params: {
            path: { todosetId },
          },
          body: {
            name: req.name,
            description: req.description,
          },
        })
    );
    return response;
  }
}