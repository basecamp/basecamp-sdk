/**
 * Doors service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Door entity from the Basecamp API. */
export type Door = components["schemas"]["Door"];

/**
 * Request parameters for create.
 */
export interface CreateDoorRequest {
  /** Door */
  door: components["schemas"]["CreateDoorPayload"];
  /** Whether the project's clients can see the external link. Omitted means
false: the link is private to the team. A client caller always creates a
client-visible link regardless of what is sent. */
  visibleToClients?: boolean;
}

/**
 * Request parameters for update.
 */
export interface UpdateDoorRequest {
  /** Door */
  door?: components["schemas"]["UpdateDoorPayload"];
  /** Whether the project's clients can see the external link. Omitted keeps
the current visibility. */
  visibleToClients?: boolean;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Doors operations.
 */
export class DoorsService extends BaseService {

  /**
   * List the active external links in a project
   * @param bucketId - The bucket ID
   * @returns Array of Door
   *
   * @example
   * ```ts
   * const result = await client.doors.list(123);
   * ```
   */
  async list(bucketId: number): Promise<Door[]> {
    const response = await this.request(
      {
        service: "Doors",
        operation: "ListDoors",
        resourceType: "door",
        isMutation: false,
        projectId: bucketId,
      },
      () =>
        this.client.GET("/buckets/{bucketId}/dock/doors.json", {
          params: {
            path: { bucketId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create an external link in a project
   * @param bucketId - The bucket ID
   * @param req - Door creation parameters
   * @returns The Door
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.doors.create(123, { door: { service: "example", url: "example" } });
   * ```
   */
  async create(bucketId: number, req: CreateDoorRequest): Promise<Door> {
    if (!req.door) {
      throw Errors.validation("Door is required");
    }
    const response = await this.request(
      {
        service: "Doors",
        operation: "CreateDoor",
        resourceType: "door",
        isMutation: true,
        projectId: bucketId,
      },
      () =>
        this.client.POST("/buckets/{bucketId}/dock/doors.json", {
          params: {
            path: { bucketId },
          },
          body: {
            door: req.door,
            visible_to_clients: req.visibleToClients,
          },
        })
    );
    return response;
  }

  /**
   * Get an external link by id
   * @param doorId - The door ID
   * @returns The Door
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.doors.get(123);
   * ```
   */
  async get(doorId: number): Promise<Door> {
    const response = await this.request(
      {
        service: "Doors",
        operation: "GetDoor",
        resourceType: "door",
        isMutation: false,
        resourceId: doorId,
      },
      () =>
        this.client.GET("/dock/doors/{doorId}", {
          params: {
            path: { doorId },
          },
        })
    );
    return response;
  }

  /**
   * Update an external link
   * @param doorId - The door ID
   * @param req - Door update parameters
   * @returns The Door
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.doors.update(123, { });
   * ```
   */
  async update(doorId: number, req: UpdateDoorRequest): Promise<Door> {
    const response = await this.request(
      {
        service: "Doors",
        operation: "UpdateDoor",
        resourceType: "door",
        isMutation: true,
        resourceId: doorId,
      },
      () =>
        this.client.PUT("/dock/doors/{doorId}", {
          params: {
            path: { doorId },
          },
          body: {
            door: req.door,
            visible_to_clients: req.visibleToClients,
          },
        })
    );
    return response;
  }

  /**
   * Delete an external link
   * @param doorId - The door ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.doors.delete(123);
   * ```
   */
  async delete(doorId: number): Promise<void> {
    await this.request(
      {
        service: "Doors",
        operation: "DeleteDoor",
        resourceType: "door",
        isMutation: true,
        resourceId: doorId,
      },
      () =>
        this.client.DELETE("/dock/doors/{doorId}", {
          params: {
            path: { doorId },
          },
        })
    );
  }
}