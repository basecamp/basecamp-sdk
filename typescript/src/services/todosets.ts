/**
 * Todosets service for Basecamp SDK.
 *
 * A todoset is the container for all todolists in a project.
 * Each project has exactly one todoset in its dock.
 */

import { BaseService } from "./base.js";
import type { components } from "../generated/schema.js";

/**
 * A todoset - the container for todolists in a project.
 * Directly uses the generated schema type.
 */
export type Todoset = components["schemas"]["Todoset"];

/**
 * A bucket (project) reference in a todoset.
 */
export type TodosetBucket = components["schemas"]["TodoBucket"];

/**
 * A person reference (creator) in a todoset.
 */
export type TodosetCreator = components["schemas"]["Person"];

/**
 * Service for todoset operations.
 *
 * @example
 * ```ts
 * // Get the todoset for a project
 * const todoset = await client.todosets.get(projectId, todosetId);
 * console.log(`${todoset.completed_count} of ${todoset.todolists_count} todolists`);
 * console.log(`Completion: ${todoset.completed_ratio}`);
 * ```
 */
export class TodosetsService extends BaseService {
  /**
   * Gets a todoset by ID.
   *
   * The todoset contains summary information about all todolists
   * in a project, including counts and completion status.
   *
   * @param projectId - The project ID (bucket ID)
   * @param todosetId - The todoset ID
   * @returns The todoset
   *
   * @example
   * ```ts
   * const todoset = await client.todosets.get(projectId, todosetId);
   *
   * console.log(`Todoset: ${todoset.name}`);
   * console.log(`${todoset.todolists_count} todolists`);
   * console.log(`Progress: ${todoset.completed_ratio}`);
   *
   * if (todoset.over_schedule_count && todoset.over_schedule_count > 0) {
   *   console.log(`Warning: ${todoset.over_schedule_count} overdue items`);
   * }
   * ```
   */
  async get(projectId: number, todosetId: number): Promise<Todoset> {
    const response = await this.request(
      {
        service: "Todosets",
        operation: "Get",
        resourceType: "todoset",
        isMutation: false,
        projectId,
        resourceId: todosetId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/todosets/{todosetId}", {
          params: { path: { projectId, todosetId } },
        })
    );

    // The response wrapper has a todoset field - return the todoset directly
    return (response as { todoset?: Todoset }).todoset ?? (response as Todoset);
  }
}
