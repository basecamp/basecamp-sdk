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
 * Request parameters for replace.
 */
export interface ReplaceTodolistRequest {
  /** Name (required for both Todolist and TodolistGroup) - presence-validated server-side, so omitting it is a 422, not a preserve */
  name: string;
  /** Description (rich text HTML) - writable for a todolist group as well as a todolist, and omitting it clears it either way */
  description?: string;
}

/**
 * Request parameters for reposition.
 */
export interface RepositionTodolistRequest {
  /** Position for ordering (1-based) */
  position: number;
}

/**
 * Options for list.
 */
export interface ListTodolistOptions extends PaginationOptions {
  /** Filter by status */
  status?: "active" | "archived" | "trashed";
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Request parameters for create.
 */
export interface CreateTodolistRequest {
  /** Display name */
  name: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Visible to clients */
  visibleToClients?: boolean;
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
        resourceId: id,
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
   * Replace a todolist (or todolist group) with a new complete representation.
   * @param id - The id
   * @param req - Todolist_or_group request parameters
   * @returns The todolist_or_group
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.todolists.replace(123, { name: "My example" });
   * ```
   */
  async replace(id: number, req: ReplaceTodolistRequest): Promise<components["schemas"]["UpdateTodolistOrGroupResponseContent"]> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "Todolists",
        operation: "UpdateTodolistOrGroup",
        resourceType: "todolist_or_group",
        isMutation: true,
        resourceId: id,
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
   * Reposition a to-do list within its to-do set.
   * @param todolistId - The todolist ID
   * @param req - Todolist request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.todolists.reposition(123, { position: 1 });
   * ```
   */
  async reposition(todolistId: number, req: RepositionTodolistRequest): Promise<void> {
    await this.request(
      {
        service: "Todolists",
        operation: "RepositionTodolist",
        resourceType: "todolist",
        isMutation: true,
        resourceId: todolistId,
      },
      () =>
        this.client.PUT("/todosets/todolists/{todolistId}/position.json", {
          params: {
            path: { todolistId },
          },
          body: {
            position: req.position,
          },
        })
    );
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
            query: { status: options?.status, page: options?.page },
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
            visible_to_clients: req.visibleToClients,
          },
        })
    );
    return response;
  }
}