/**
 * Pings service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Request parameters for create.
 */
export interface CreatePingRequest {
  /** Person ids of the people to ping. People the caller cannot reach are
dropped. */
  userIds: number[];
  /** Create a new group even when a Ping with exactly these people exists. */
  newGroup?: boolean;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Pings operations.
 */
export class PingsService extends BaseService {

  /**
   * Get Pings: the caller's active Pings that have a line in them, most
   * @returns Array of results
   *
   * @example
   * ```ts
   * const result = await client.pings.list();
   * ```
   */
  async list(): Promise<components["schemas"]["ListPingsResponseContent"]> {
    const response = await this.request(
      {
        service: "Pings",
        operation: "ListPings",
        resourceType: "ping",
        isMutation: false,
      },
      () =>
        this.client.GET("/circles.json", {
        })
    );
    return response ?? [];
  }

  /**
   * Create a Ping with people, or return the one that already holds exactly
   * @param req - Ping creation parameters
   * @returns The ping
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.pings.create({ userIds: [1234] });
   * ```
   */
  async create(req: CreatePingRequest): Promise<components["schemas"]["CreatePingResponseContent"]> {
    if (!req.userIds) {
      throw Errors.validation("User ids is required");
    }
    const response = await this.request(
      {
        service: "Pings",
        operation: "CreatePing",
        resourceType: "ping",
        isMutation: true,
      },
      () =>
        this.client.POST("/circles.json", {
          body: {
            user_ids: req.userIds,
            new_group: req.newGroup,
          },
        })
    );
    return response;
  }

  /**
   * Get a Ping by its id, which is its bucket's id.
   * @param pingId - A Ping's id is the id of the bucket that holds it (the bucket type is
"Circle", which is also the wire path), so the `bucket.id` a Ping's
recordings carry is the id GetPing takes.
   * @returns The ping
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.pings.get(123);
   * ```
   */
  async get(pingId: number): Promise<components["schemas"]["GetPingResponseContent"]> {
    const response = await this.request(
      {
        service: "Pings",
        operation: "GetPing",
        resourceType: "ping",
        isMutation: false,
        resourceId: pingId,
      },
      () =>
        this.client.GET("/circles/{pingId}", {
          params: {
            path: { pingId },
          },
        })
    );
    return response;
  }
}