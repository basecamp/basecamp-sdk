/**
 * Todosets service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Todoset entity from the Basecamp API. */
export type Todoset = components["schemas"]["Todoset"];

// =============================================================================
// Service
// =============================================================================

/**
 * Service for Todosets operations.
 */
export class TodosetsService extends BaseService {

  /**
   * Get a todoset (container for todolists in a project)
   * @param projectId - The project ID
   * @param todosetId - The todoset ID
   * @returns The Todoset
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.todosets.get(123, 123);
   * ```
   */
  async get(projectId: number, todosetId: number): Promise<Todoset> {
    const response = await this.request(
      {
        service: "Todosets",
        operation: "GetTodoset",
        resourceType: "todoset",
        isMutation: false,
        projectId,
        resourceId: todosetId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todosets/{todosetId}", {
          params: {
            path: { projectId, todosetId },
          },
        })
    );
    return response;
  }
}